package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pubg/kube-image-deployer/imageNotifier"
	"github.com/pubg/kube-image-deployer/remoteRegistry/docker"
	"github.com/pubg/kube-image-deployer/watcher"
	appV1 "k8s.io/api/apps/v1"
	batchV1beta1 "k8s.io/api/batch/v1beta1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	kubeconfig               = *flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	offDeployments           = *flag.Bool("off-deployments", false, "disable deployments")
	offStatefulsets          = *flag.Bool("off-statefulsets", false, "disable statefulsets")
	offDaemonsets            = *flag.Bool("off-daemonsets", false, "disable daemonsets")
	offCronjobs              = *flag.Bool("off-cronjobs", false, "disable cronjobs")
	imageStringCacheTTLSec   = *flag.Uint("image-hash-cache-ttl-sec", 60, "image hash cache TTL in seconds")
	imageCheckIntervalSec    = *flag.Uint("image-check-interval-sec", 10, "image check interval in seconds")
	controllerWatchKey       = *flag.String("controller-watch-key", "kube-image-deployer", "controller watch key")
	controllerWatchNamespace = *flag.String("controller-watch-namespace", "", "controller watch namespace. If empty, watch all namespaces")
	imageDefaultPlatform     = *flag.String("image-default-platform", "linux/amd64", "default platform for docker images")
)

func init() {
	klog.InitFlags(nil)
	flag.Parse()

	klog.Infof("Starting pid: %d", os.Getpid())
	klog.Infof("Config Flags: %v", map[string]interface{}{
		"kubeconfig":               kubeconfig,
		"offDeployments":           offDeployments,
		"offStatefulsets":          offStatefulsets,
		"offDaemonsets":            offDaemonsets,
		"offCronjobs":              offCronjobs,
		"imageStringCacheTTLSec":   imageStringCacheTTLSec,
		"imageCheckIntervalSec":    imageCheckIntervalSec,
		"controllerWatchKey":       controllerWatchKey,
		"controllerWatchNamespace": controllerWatchNamespace,
	})
}

// NewClientset returns a new kubernetes clientset
func NewClientset() *kubernetes.Clientset {

	// try the in-cluster config
	if config, err := rest.InClusterConfig(); err == nil {
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			klog.Fatal(err)
		}
		return clientset
	}

	home, _ := os.UserHomeDir()

	if kubeconfig == "" && home != "" {
		kubeconfig = home + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)

	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		klog.Fatal(err)
	}

	return clientset
}

func runWatchers(stopCh chan struct{}) {
	clientset := NewClientset()                                                                    // create a clientset
	remoteRegistry := docker.NewRemoteRegistry().WithDefaultPlatform(imageDefaultPlatform)         // create a docker remote registry
	imageNotifier := imageNotifier.NewImageNotifier(stopCh, remoteRegistry, imageCheckIntervalSec) // create a imageNotifier
	optionsModifier := func(options *metaV1.ListOptions) {                                         // optionsModifier selector
		options.LabelSelector = controllerWatchKey
	}

	if !offDeployments { // deployments watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().Deployments(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		go watcher.NewWatcher("deployments", stopCh, cache.NewFilteredListWatchFromClient(clientset.AppsV1().RESTClient(), "deployments", controllerWatchNamespace, optionsModifier), &appV1.Deployment{}, imageNotifier, controllerWatchKey, applyStrategicMergePatch)
	}

	if !offStatefulsets { // statefulsets watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().StatefulSets(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		go watcher.NewWatcher("statefulsets", stopCh, cache.NewFilteredListWatchFromClient(clientset.AppsV1().RESTClient(), "statefulsets", controllerWatchNamespace, optionsModifier), &appV1.StatefulSet{}, imageNotifier, controllerWatchKey, applyStrategicMergePatch)
	}

	if !offDaemonsets { // daemonsets watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().DaemonSets(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		go watcher.NewWatcher("daemonsets", stopCh, cache.NewFilteredListWatchFromClient(clientset.AppsV1().RESTClient(), "daemonsets", controllerWatchNamespace, optionsModifier), &appV1.DaemonSet{}, imageNotifier, controllerWatchKey, applyStrategicMergePatch)
	}

	if !offCronjobs { // cronjobs watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.BatchV1beta1().CronJobs(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		go watcher.NewWatcher("cronjobs", stopCh, cache.NewFilteredListWatchFromClient(clientset.BatchV1beta1().RESTClient(), "cronjobs", controllerWatchNamespace, optionsModifier), &batchV1beta1.CronJob{}, imageNotifier, controllerWatchKey, applyStrategicMergePatch)
	}
}

func main() {

	sigs := make(chan os.Signal, 1)
	stopCh := make(chan struct{})
	// defer close(stop)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// run watchers
	runWatchers(stopCh)

	// wait for a signal
	go func() {
		sig := <-sigs
		klog.Warningf("Signal (%s) received, stopping", sig)
		close(stopCh)
	}()

	<-stopCh
	<-time.After(10 * time.Second) // wait for the watchers to stop

}
