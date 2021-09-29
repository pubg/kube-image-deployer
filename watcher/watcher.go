package watcher

import (
	"github.com/pubg/kube-image-deployer/controller"
	"github.com/pubg/kube-image-deployer/interfaces"
	l "github.com/pubg/kube-image-deployer/logger"
	pkgRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type ApplyStrategicMergePatch = controller.ApplyStrategicMergePatch

func NewWatcher(
	name string,
	stop chan struct{},
	logger interfaces.ILogger,
	listWatcher cache.ListerWatcher,
	objType pkgRuntime.Object,
	imageNotifier interfaces.IImageNotifier,
	controllerWatchKey string,
	applyStrategicMergePatch ApplyStrategicMergePatch,
) {
	controller := createDefaultController(name, stop, logger, listWatcher, objType, imageNotifier, controllerWatchKey, applyStrategicMergePatch)
	RunController(stop, controller)
}

func RunController(
	stop chan struct{},
	controller interfaces.IController,
) {
	controller.Run(1, stop) // Let's start the controller
}

func createDefaultController(
	name string,
	stop chan struct{},
	logger interfaces.ILogger,
	listWatcher cache.ListerWatcher,
	objType pkgRuntime.Object,
	imageNotifier interfaces.IImageNotifier,
	controllerWatchKey string,
	applyStrategicMergePatch ApplyStrategicMergePatch,
) *controller.Controller {

	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Bind the workqueue to a cache with the help of an informer. This way we make sure that
	// whenever the cache is updated, the pod key is added to the workqueue.
	// Note that when we finally process the item from the workqueue, we might see a newer version
	// of the Pod than the version which was responsible for triggering the update.
	indexer, informer := cache.NewIndexerInformer(listWatcher, objType, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})

	controllerOpt := controller.ControllerOpt{
		Resource:                 name,
		ObjType:                  objType,
		ApplyStrategicMergePatch: applyStrategicMergePatch,
		Queue:                    queue,
		Indexer:                  indexer,
		Informer:                 informer,
		ImageNotifier:            imageNotifier,
		ControllerWatchKey:       controllerWatchKey,
		Logger:                   logger,
	}

	if controllerOpt.Logger == nil {
		controllerOpt.Logger = l.NewLogger()
	}

	controller := controller.NewController(controllerOpt)

	return controller

}
