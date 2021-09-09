package watcher

import (
	"github.com/pubg/kube-image-deployer/controller"
	"github.com/pubg/kube-image-deployer/interfaces"
	pkgRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func NewWatcher(
	name string,
	stop chan struct{},
	clientset *kubernetes.Clientset,
	listWatcher cache.ListerWatcher,
	objType pkgRuntime.Object,
	imageNotifier interfaces.IImageNotifier,
	controllerWatchKey string,
) {
	NewWatcherWithController(stop, createDefaultController(name, stop, clientset, listWatcher, objType, imageNotifier, controllerWatchKey))
}

func NewWatcherWithController(
	stop chan struct{},
	controller interfaces.IController,
) {
	go controller.Run(1, stop) // Let's start the controller
	<-stop
}

func createDefaultController(
	name string,
	stop chan struct{},
	clientset *kubernetes.Clientset,
	listWatcher cache.ListerWatcher,
	objType pkgRuntime.Object,
	imageNotifier interfaces.IImageNotifier,
	controllerWatchKey string,
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

	controller := controller.NewController(name, clientset, objType, queue, indexer, informer, imageNotifier, controllerWatchKey)

	return controller

}
