package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/pubg/kube-image-deployer/logger"
	"github.com/pubg/kube-image-deployer/watcher"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
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
		"imageStringCacheTTLSec":   imageStringCacheTTLSec,
		"imageCheckIntervalSec":    imageCheckIntervalSec,
		"controllerWatchKey":       controllerWatchKey,
		"controllerWatchNamespace": controllerWatchNamespace,
		"slackWebhook":             slackWebhook,
		"slackMsgPrefix":           slackMsgPrefix,
	})
}

// newClientset returns a new kubernetes clientset
func newClientset() *kubernetes.Clientset {

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

func newLogger(stopCh chan struct{}) *logger.Logger {
	logger := logger.NewLogger()

	if slackWebhook != "" {
		logger.WithSlack(stopCh, slackWebhook, slackMsgPrefix)
	}
	return logger
}

func main() {

	sigs := make(chan os.Signal, 1)
	stopCh := make(chan struct{})

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// run watchers
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	var wg sync.WaitGroup

	clientset := newClientset()
	logger := newLogger(stopCh)

	opt := &watcher.RunOptions{
		OffDeployments:           offDeployments,
		OffStatefulsets:          offStatefulsets,
		OffDaemonsets:            offDaemonsets,
		OffCronjobs:              offCronjobs,
		ImageStringCacheTTLSec:   imageStringCacheTTLSec,
		ImageCheckIntervalSec:    imageCheckIntervalSec,
		ControllerWatchKey:       controllerWatchKey,
		ControllerWatchNamespace: controllerWatchNamespace,
		ImageDefaultPlatform:     imageDefaultPlatform,
	}

	watcher.Run(opt, ctx, clientset, stopCh, &wg, logger)

	// wait for a signal
	go func() {
		sig := <-sigs
		klog.Warningf("Signal (%s) received, stopping", sig)
		cancelFn()
		close(stopCh)
	}()

	<-stopCh
	wg.Wait()

}
