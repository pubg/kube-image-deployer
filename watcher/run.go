package watcher

import (
	"context"
	"sync"

	"github.com/pubg/kube-image-deployer/events"
	"github.com/pubg/kube-image-deployer/imageNotifier"
	"github.com/pubg/kube-image-deployer/logger"
	"github.com/pubg/kube-image-deployer/remoteRegistry/docker"
	appV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type RunOptions struct {
	OffDeployments           bool
	OffStatefulsets          bool
	OffDaemonsets            bool
	OffCronjobs              bool
	ImageStringCacheTTLSec   uint
	ImageCheckIntervalSec    uint
	ControllerWatchKey       string
	ControllerWatchNamespace string
	ImageDefaultPlatform     string
	ImagePatchedSink         events.Sink // 이미지 패치 구조화 이벤트 수신자 (nil이면 비활성)
}

// Run 활성화된 workload 타입별 컨트롤러를 생성해 goroutine으로 실행한다.
// 반환값은 readiness probe에서 사용할 informer 동기화 확인 함수 목록이다.
func Run(ctx context.Context, opt *RunOptions, clientset *kubernetes.Clientset, wg *sync.WaitGroup, logger *logger.Logger) []cache.InformerSynced {

	remoteRegistry := docker.NewRemoteRegistry().WithDefaultPlatform(opt.ImageDefaultPlatform).WithCache(opt.ImageStringCacheTTLSec).WithLogger(logger) // create a docker remote registry
	notifier := imageNotifier.NewImageNotifier(ctx, remoteRegistry, opt.ImageCheckIntervalSec).WithLogger(logger)                                       // create a imageNotifier
	optionsModifier := func(options *metaV1.ListOptions) {                                                                                              // optionsModifier selector
		options.LabelSelector = opt.ControllerWatchKey
	}

	synced := make([]cache.InformerSynced, 0, 4)

	startController := func(name string, restClient rest.Interface, objType pkgRuntime.Object, applyStrategicMergePatch ApplyStrategicMergePatch) {
		c := createDefaultController(
			name,
			logger,
			cache.NewFilteredListWatchFromClient(restClient, name, opt.ControllerWatchNamespace, optionsModifier),
			objType,
			notifier,
			opt.ControllerWatchKey,
			applyStrategicMergePatch,
			opt.ImagePatchedSink,
		)
		synced = append(synced, c.HasSynced)
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Run(ctx, 1)
		}()
	}

	if !opt.OffDeployments { // deployments watcher
		startController("deployments", clientset.AppsV1().RESTClient(), &appV1.Deployment{}, func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().Deployments(namespace).Patch(ctx, name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		})
	}

	if !opt.OffStatefulsets { // statefulsets watcher
		startController("statefulsets", clientset.AppsV1().RESTClient(), &appV1.StatefulSet{}, func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().StatefulSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		})
	}

	if !opt.OffDaemonsets { // daemonsets watcher
		startController("daemonsets", clientset.AppsV1().RESTClient(), &appV1.DaemonSet{}, func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().DaemonSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		})
	}

	if !opt.OffCronjobs { // cronjobs watcher
		startController("cronjobs", clientset.BatchV1().RESTClient(), &batchV1.CronJob{}, func(namespace string, name string, data []byte) error {
			_, err := clientset.BatchV1().CronJobs(namespace).Patch(ctx, name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		})
	}

	return synced
}
