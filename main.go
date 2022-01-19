package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pubg/kube-image-deployer/imageNotifier"
	"github.com/pubg/kube-image-deployer/logger"
	"github.com/pubg/kube-image-deployer/remoteRegistry/docker"
	"github.com/pubg/kube-image-deployer/watcher"
	appV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
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
	useCronJobV1             = *flag.Bool("use-cronjob-v1", false, "use cronjob version v1 instead of v1beta1")
	imageStringCacheTTLSec   = *flag.Uint("image-hash-cache-ttl-sec", 60, "image hash cache TTL in seconds")
	imageCheckIntervalSec    = *flag.Uint("image-check-interval-sec", 10, "image check interval in seconds")
	controllerWatchKey       = *flag.String("controller-watch-key", "kube-image-deployer", "controller watch key")
	controllerWatchNamespace = *flag.String("controller-watch-namespace", "", "controller watch namespace. If empty, watch all namespaces")
	imageDefaultPlatform     = *flag.String("image-default-platform", "linux/amd64", "default platform for docker images")
	slackWebhook             = *flag.String("slack-webhook", "", "slack webhook url. If empty, notifications are disabled")
	slackMsgPrefix           = *flag.String("slack-msg-prefix", "["+getHostname()+"]", "slack message prefix. default=[hostname]")
)

func getHostname() string {
	if s, err := os.Hostname(); err == nil {
		return s
	}
	return "unknown"
}

func init() {
	klog.InitFlags(nil)
	flag.Parse()
	klog.Infof("Starting pid: %d", os.Getpid())
	godotenv.Load(".env")

	if os.Getenv("KUBECONFIG_PATH") != "" {
		kubeconfig = os.Getenv("KUBECONFIG_PATH")
	}
	if os.Getenv("OFF_DEPLOYMENTS") != "" {
		offDeployments = true
	}
	if os.Getenv("OFF_STATEFULSETS") != "" {
		offStatefulsets = true
	}
	if os.Getenv("OFF_DAEMONSETS") != "" {
		offDaemonsets = true
	}
	if os.Getenv("OFF_CRONJOBS") != "" {
		offCronjobs = true
	}
	if os.Getenv("USE_CRONJOB_V1") != "" {
		useCronJobV1 = true
	}
	if os.Getenv("IMAGE_HASH_CACHE_TTL_SEC") != "" {
		if v, err := strconv.ParseUint(os.Getenv("IMAGE_HASH_CACHE_TTL_SEC"), 10, 32); err == nil {
			imageStringCacheTTLSec = uint(v)
		}
	}
	if os.Getenv("IMAGE_CHECK_INTERVAL_SEC") != "" {
		if v, err := strconv.ParseUint(os.Getenv("IMAGE_CHECK_INTERVAL_SEC"), 10, 32); err == nil {
			imageCheckIntervalSec = uint(v)
		}
	}
	if os.Getenv("CONTROLLER_WATCH_KEY") != "" {
		controllerWatchKey = os.Getenv("CONTROLLER_WATCH_KEY")
	}
	if os.Getenv("CONTROLLER_WATCH_NAMESPACE") != "" {
		controllerWatchNamespace = os.Getenv("CONTROLLER_WATCH_NAMESPACE")
	}
	if os.Getenv("IMAGE_DEFAULT_PLATFORM") != "" {
		imageDefaultPlatform = os.Getenv("IMAGE_DEFAULT_PLATFORM")
	}
	if os.Getenv("SLACK_WEBHOOK") != "" {
		slackWebhook = os.Getenv("SLACK_WEBHOOK")
	}
	if os.Getenv("SLACK_MSG_PREFIX") != "" {
		slackMsgPrefix = os.Getenv("SLACK_MSG_PREFIX")
	}

	klog.Infof("Config Flags: %v", map[string]interface{}{
		"kubeconfig":               kubeconfig,
		"offDeployments":           offDeployments,
		"offStatefulsets":          offStatefulsets,
		"offDaemonsets":            offDaemonsets,
		"offCronjobs":              offCronjobs,
		"useCronJobV1":             useCronJobV1,
		"imageStringCacheTTLSec":   imageStringCacheTTLSec,
		"imageCheckIntervalSec":    imageCheckIntervalSec,
		"controllerWatchKey":       controllerWatchKey,
		"controllerWatchNamespace": controllerWatchNamespace,
		"slackWebhook":             slackWebhook,
		"slackMsgPrefix":           slackMsgPrefix,
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
	logger := logger.NewLogger()

	if slackWebhook != "" {
		logger.WithSlack(stopCh, slackWebhook, slackMsgPrefix)
	}

	clientset := NewClientset()                                                                                       // create a clientset
	remoteRegistry := docker.NewRemoteRegistry().WithDefaultPlatform(imageDefaultPlatform).WithLogger(logger)         // create a docker remote registry
	imageNotifier := imageNotifier.NewImageNotifier(stopCh, remoteRegistry, imageCheckIntervalSec).WithLogger(logger) // create a imageNotifier
	optionsModifier := func(options *metaV1.ListOptions) {                                                            // optionsModifier selector
		options.LabelSelector = controllerWatchKey
	}

	if !offDeployments { // deployments watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().Deployments(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		go watcher.NewWatcher("deployments", stopCh, logger, cache.NewFilteredListWatchFromClient(clientset.AppsV1().RESTClient(), "deployments", controllerWatchNamespace, optionsModifier), &appV1.Deployment{}, imageNotifier, controllerWatchKey, applyStrategicMergePatch)
	}

	if !offStatefulsets { // statefulsets watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().StatefulSets(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		go watcher.NewWatcher("statefulsets", stopCh, logger, cache.NewFilteredListWatchFromClient(clientset.AppsV1().RESTClient(), "statefulsets", controllerWatchNamespace, optionsModifier), &appV1.StatefulSet{}, imageNotifier, controllerWatchKey, applyStrategicMergePatch)
	}

	if !offDaemonsets { // daemonsets watcher
		applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
			_, err := clientset.AppsV1().DaemonSets(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
			return err
		}
		go watcher.NewWatcher("daemonsets", stopCh, logger, cache.NewFilteredListWatchFromClient(clientset.AppsV1().RESTClient(), "daemonsets", controllerWatchNamespace, optionsModifier), &appV1.DaemonSet{}, imageNotifier, controllerWatchKey, applyStrategicMergePatch)
	}

	if !offCronjobs { // cronjobs watcher
		if useCronJobV1 {
			applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
				_, err := clientset.BatchV1().CronJobs(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
				return err
			}
			go watcher.NewWatcher("cronjobs", stopCh, logger, cache.NewFilteredListWatchFromClient(clientset.BatchV1().RESTClient(), "cronjobs", controllerWatchNamespace, optionsModifier), &batchV1.CronJob{}, imageNotifier, controllerWatchKey, applyStrategicMergePatch)
		} else {
			applyStrategicMergePatch := func(namespace string, name string, data []byte) error {
				_, err := clientset.BatchV1beta1().CronJobs(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, data, metaV1.PatchOptions{})
				return err
			}
			go watcher.NewWatcher("cronjobs", stopCh, logger, cache.NewFilteredListWatchFromClient(clientset.BatchV1beta1().RESTClient(), "cronjobs", controllerWatchNamespace, optionsModifier), &batchV1beta1.CronJob{}, imageNotifier, controllerWatchKey, applyStrategicMergePatch)
		}
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
