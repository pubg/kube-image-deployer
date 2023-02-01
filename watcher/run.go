package watcher

import (
	"context"
	"sync"

	"github.com/pubg/kube-image-deployer/imageNotifier"
	"github.com/pubg/kube-image-deployer/logger"
	"github.com/pubg/kube-image-deployer/remoteRegistry/docker"
	appV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
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
}

func Run(opt *RunOptions, ctx context.Context, clientset *kubernetes.Clientset, stopCh chan struct{}, wg *sync.WaitGroup, logger *logger.Logger) {

	remoteRegistry := docker.NewRemoteRegistry().WithDefaultPlatform(opt.ImageDefaultPlatform).WithLogger(logger)         // create a docker remote registry
	imageNotifier := imageNotifier.NewImageNotifier(stopCh, remoteRegistry, opt.ImageCheckIntervalSec).WithLogger(logger) // create a imageNotifier
	optionsModifier := func(options *metaV1.ListOptions) {                                                                // optionsModifier selector
		options.LabelSelector = opt.ControllerWatchKey
	}

	if !opt.OffDeployments { // deployments watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().Deployments(namespace).Patch(ctx, name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			newWatcher("deployments", stopCh, logger, cache.NewFilteredListWatchFromClient(clientset.AppsV1().RESTClient(), "deployments", opt.ControllerWatchNamespace, optionsModifier), &appV1.Deployment{}, imageNotifier, opt.ControllerWatchKey, applyStrategicMergePatch)
		}()
	}

	if !opt.OffStatefulsets { // statefulsets watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().StatefulSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			newWatcher("statefulsets", stopCh, logger, cache.NewFilteredListWatchFromClient(clientset.AppsV1().RESTClient(), "statefulsets", opt.ControllerWatchNamespace, optionsModifier), &appV1.StatefulSet{}, imageNotifier, opt.ControllerWatchKey, applyStrategicMergePatch)
		}()
	}

	if !opt.OffDaemonsets { // daemonsets watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().DaemonSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			newWatcher("daemonsets", stopCh, logger, cache.NewFilteredListWatchFromClient(clientset.AppsV1().RESTClient(), "daemonsets", opt.ControllerWatchNamespace, optionsModifier), &appV1.DaemonSet{}, imageNotifier, opt.ControllerWatchKey, applyStrategicMergePatch)
		}()
	}

	if !opt.OffCronjobs { // cronjobs watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.BatchV1().CronJobs(namespace).Patch(ctx, name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			newWatcher("cronjobs", stopCh, logger, cache.NewFilteredListWatchFromClient(clientset.BatchV1().RESTClient(), "cronjobs", opt.ControllerWatchNamespace, optionsModifier), &batchV1.CronJob{}, imageNotifier, opt.ControllerWatchKey, applyStrategicMergePatch)
		}()
	}
}
