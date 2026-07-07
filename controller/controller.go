package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pubg/kube-image-deployer/events"
	"github.com/pubg/kube-image-deployer/interfaces"

	pkgRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Controller demonstrates how to implement a controller with client-go.
type Controller struct {
	resource                 string
	objType                  pkgRuntime.Object
	indexer                  cache.Indexer
	queue                    workqueue.TypedRateLimitingInterface[string]
	informer                 cache.Controller
	imageNotifier            interfaces.IImageNotifier
	applyStrategicMergePatch ApplyStrategicMergePatch
	logger                   interfaces.ILogger
	imagePatchedSink         events.Sink // 패치 성공 시 구조화 이벤트 발행 (nil이면 비활성)

	syncedImages      map[Image]bool
	syncedImagesMutex sync.RWMutex

	// desiredImages imageNotifier가 통지한 url:tag별 최신 이미지(digest + 취득 태그).
	// syncKey(reconcile)에서 workload의 현재 이미지와 비교해 패치를 적용한다.
	desiredImages      map[imageKey]interfaces.ResolvedImage
	desiredImagesMutex sync.RWMutex

	watchKey string
}

type ApplyStrategicMergePatch func(namespace, name string, data []byte) error

type ControllerOpt struct {
	Resource                 string
	ObjType                  pkgRuntime.Object
	Indexer                  cache.Indexer
	Informer                 cache.Controller
	Queue                    workqueue.TypedRateLimitingInterface[string]
	ImageNotifier            interfaces.IImageNotifier
	ApplyStrategicMergePatch ApplyStrategicMergePatch
	ControllerWatchKey       string
	Logger                   interfaces.ILogger
	ImagePatchedSink         events.Sink
}

// NewController creates a new Controller.
func NewController(opt ControllerOpt) *Controller {
	return &Controller{
		resource:                 opt.Resource,
		objType:                  opt.ObjType,
		indexer:                  opt.Indexer,
		informer:                 opt.Informer,
		queue:                    opt.Queue,
		imageNotifier:            opt.ImageNotifier,
		applyStrategicMergePatch: opt.ApplyStrategicMergePatch,
		watchKey:                 opt.ControllerWatchKey,
		logger:                   opt.Logger,
		imagePatchedSink:         opt.ImagePatchedSink,
		syncedImages:             make(map[Image]bool),
		syncedImagesMutex:        sync.RWMutex{},
		desiredImages:            make(map[imageKey]interfaces.ResolvedImage),
		desiredImagesMutex:       sync.RWMutex{},
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	err := c.syncKey(key)
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)
	return true
}

// maxErrorLogRetries 반복 실패를 Error 레벨로 알리는 횟수 상한. 이후에는 Debugf(klog 전용)로 강등.
const maxErrorLogRetries = 5

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key string) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// desired state가 남아있는 한 재시도를 포기하면 이미지가 영구히 미반영 상태로 남으므로,
	// rate limiter의 지수 백오프(기본 5ms~1000s)와 함께 무기한 재시도한다.
	// 처음 몇 회만 Error(Slack 미러 포함)로 알리고, 이후 반복 실패는 klog에만 남겨
	// 영구 실패가 백오프 상한 간격으로 Slack을 계속 울리지 않게 한다.
	if c.queue.NumRequeues(key) < maxErrorLogRetries {
		c.logger.Errorf("[%s] Error syncing %v, requeuing with backoff (retries=%d): %v", c.resource, key, c.queue.NumRequeues(key), err)
	} else {
		c.logger.Debugf("[%s] Error syncing %v, requeuing with backoff (retries=%d): %v", c.resource, key, c.queue.NumRequeues(key), err)
	}
	c.queue.AddRateLimited(key)
}

// Run begins watching and syncing.
func (c *Controller) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	c.logger.Infof("[%s] Starting controller", c.resource)

	go c.informer.Run(ctx.Done())

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("[%s] : Timed out waiting for caches to sync", c.resource))
		return
	}

	for range workers {
		go wait.Until(c.runWorker, time.Second, ctx.Done())
	}

	<-ctx.Done()
	c.logger.Infof("[%s] Stopping controller", c.resource)
}

// HasSynced informer 캐시 동기화 여부. readiness probe에서 사용한다.
func (c *Controller) HasSynced() bool {
	return c.informer.HasSynced()
}

func (c *Controller) GetResourceName() string {
	return c.resource
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}
