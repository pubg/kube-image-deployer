package controller

import (
	"fmt"
	"sync"
	"time"

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
	queue                    workqueue.RateLimitingInterface
	informer                 cache.Controller
	imageNotifier            interfaces.IImageNotifier
	applyStrategicMergePatch ApplyStrategicMergePatch
	logger                   interfaces.ILogger

	syncedImages      map[Image]bool
	syncedImagesMutex sync.RWMutex

	imageUpdateNotifyList      []imageUpdateNotify
	imageUpdateNotifyListMutex sync.RWMutex

	watchKey string
}

type ApplyStrategicMergePatch func(namespace, name string, data []byte) error

type ControllerOpt struct {
	Resource                 string
	ObjType                  pkgRuntime.Object
	Indexer                  cache.Indexer
	Informer                 cache.Controller
	Queue                    workqueue.RateLimitingInterface
	ImageNotifier            interfaces.IImageNotifier
	ApplyStrategicMergePatch ApplyStrategicMergePatch
	ControllerWatchKey       string
	Logger                   interfaces.ILogger
}

// NewController creates a new Controller.
func NewController(opt ControllerOpt) *Controller {
	return &Controller{
		resource:                   opt.Resource,
		objType:                    opt.ObjType,
		indexer:                    opt.Indexer,
		informer:                   opt.Informer,
		queue:                      opt.Queue,
		imageNotifier:              opt.ImageNotifier,
		applyStrategicMergePatch:   opt.ApplyStrategicMergePatch,
		watchKey:                   opt.ControllerWatchKey,
		logger:                     opt.Logger,
		syncedImages:               make(map[Image]bool),
		syncedImagesMutex:          sync.RWMutex{},
		imageUpdateNotifyList:      make([]imageUpdateNotify, 0),
		imageUpdateNotifyListMutex: sync.RWMutex{},
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
	err := c.syncKey(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)
	return true
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		c.logger.Infof("[%s] Error syncing %v: %v", c.resource, key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	c.logger.Infof("[%s] Dropping out of the queue: key:%q, err:%v", c.resource, key, err)
}

// Run begins watching and syncing.
func (c *Controller) Run(workers int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	c.logger.Infof("[%s] Starting controller", c.resource)

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("[%s] : Timed out waiting for caches to sync", c.resource))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	go wait.Until(c.patchUpdateNotifyList, time.Second, stopCh)

	<-stopCh
	c.logger.Infof("[%s] Stopping controller", c.resource)
}

func (c *Controller) GetReresourceName() string {
	return c.resource
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}
