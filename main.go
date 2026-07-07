package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pubg/kube-image-deployer/events"
	"github.com/pubg/kube-image-deployer/health"
	"github.com/pubg/kube-image-deployer/logger"
	"github.com/pubg/kube-image-deployer/util"
	"github.com/pubg/kube-image-deployer/watcher"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	flagKubeconfig               = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	flagOffDeployments           = flag.Bool("off-deployments", false, "disable deployments")
	flagOffStatefulsets          = flag.Bool("off-statefulsets", false, "disable statefulsets")
	flagOffDaemonsets            = flag.Bool("off-daemonsets", false, "disable daemonsets")
	flagOffCronjobs              = flag.Bool("off-cronjobs", false, "disable cronjobs")
	flagImageStringCacheTTLSec   = flag.Uint("image-hash-cache-ttl-sec", 60, "image hash cache TTL in seconds")
	flagImageCheckIntervalSec    = flag.Uint("image-check-interval-sec", 10, "image check interval in seconds")
	flagControllerWatchKey       = flag.String("controller-watch-key", "kube-image-deployer", "controller watch key")
	flagControllerWatchNamespace = flag.String("controller-watch-namespace", "", "controller watch namespace. If empty, watch all namespaces")
	flagImageDefaultPlatform     = flag.String("image-default-platform", "linux/amd64", "default platform for docker images")
	flagSlackWebhook             = flag.String("slack-webhook", "", "slack webhook url. If empty, notifications are disabled")
	flagSlackMsgPrefix           = flag.String("slack-msg-prefix", "["+getHostname()+"]", "slack message prefix. default=[hostname]")
	flagBindAddress              = flag.String("bind-address", ":8080", "address for /healthz, /readyz, /metrics endpoints. If empty, the http server is disabled")
	flagClusterName              = flag.String("cluster-name", getHostname(), "cluster name shown in slack image update events. default=hostname")
	flagSlackEventWebhook        = flag.String("slack-event-webhook", "", "slack webhook url for aggregated image update events. If empty, falls back to slack-webhook")
	flagSlackEventWindowSec      = flag.Uint("slack-event-window-sec", 20, "aggregation window in seconds for image update events. 0 sends immediately without batching")

	kubeconfig               string
	offDeployments           bool
	offStatefulsets          bool
	offDaemonsets            bool
	offCronjobs              bool
	imageStringCacheTTLSec   uint
	imageCheckIntervalSec    uint
	controllerWatchKey       string
	controllerWatchNamespace string
	imageDefaultPlatform     string
	slackWebhook             string
	slackMsgPrefix           string
	bindAddress              string
	clusterName              string
	slackEventWebhook        string
	slackEventWindowSec      uint
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

	// flag.Parse() 이후에 역참조해야 커맨드라인 값이 반영된다
	kubeconfig = *flagKubeconfig
	offDeployments = *flagOffDeployments
	offStatefulsets = *flagOffStatefulsets
	offDaemonsets = *flagOffDaemonsets
	offCronjobs = *flagOffCronjobs
	imageStringCacheTTLSec = *flagImageStringCacheTTLSec
	imageCheckIntervalSec = *flagImageCheckIntervalSec
	controllerWatchKey = *flagControllerWatchKey
	controllerWatchNamespace = *flagControllerWatchNamespace
	imageDefaultPlatform = *flagImageDefaultPlatform
	slackWebhook = *flagSlackWebhook
	slackMsgPrefix = *flagSlackMsgPrefix
	bindAddress = *flagBindAddress
	clusterName = *flagClusterName
	slackEventWebhook = *flagSlackEventWebhook
	slackEventWindowSec = *flagSlackEventWindowSec

	// 환경변수가 설정된 경우 flag 값을 덮어쓴다
	kubeconfig = util.GetEnvString("KUBECONFIG_PATH", kubeconfig)
	offDeployments = util.GetEnvBool("OFF_DEPLOYMENTS", offDeployments)
	offStatefulsets = util.GetEnvBool("OFF_STATEFULSETS", offStatefulsets)
	offDaemonsets = util.GetEnvBool("OFF_DAEMONSETS", offDaemonsets)
	offCronjobs = util.GetEnvBool("OFF_CRONJOBS", offCronjobs)
	imageStringCacheTTLSec = util.GetEnvUint("IMAGE_HASH_CACHE_TTL_SEC", imageStringCacheTTLSec)
	imageCheckIntervalSec = util.GetEnvUint("IMAGE_CHECK_INTERVAL_SEC", imageCheckIntervalSec)
	controllerWatchKey = util.GetEnvString("CONTROLLER_WATCH_KEY", controllerWatchKey)
	controllerWatchNamespace = util.GetEnvString("CONTROLLER_WATCH_NAMESPACE", controllerWatchNamespace)
	imageDefaultPlatform = util.GetEnvString("IMAGE_DEFAULT_PLATFORM", imageDefaultPlatform)
	slackWebhook = util.GetEnvString("SLACK_WEBHOOK", slackWebhook)
	slackMsgPrefix = util.GetEnvString("SLACK_MSG_PREFIX", slackMsgPrefix)
	bindAddress = util.GetEnvString("BIND_ADDRESS", bindAddress)
	clusterName = util.GetEnvString("CLUSTER_NAME", clusterName)
	slackEventWebhook = util.GetEnvString("SLACK_EVENT_WEBHOOK", slackEventWebhook)
	slackEventWindowSec = util.GetEnvUint("SLACK_EVENT_WINDOW_SEC", slackEventWindowSec)

	if slackEventWebhook == "" { // 이벤트 전용 webhook 미설정 시 로그 webhook으로 폴백
		slackEventWebhook = slackWebhook
	}

	klog.Infof("Config Flags: %v", map[string]any{
		"kubeconfig":               kubeconfig,
		"offDeployments":           offDeployments,
		"offStatefulsets":          offStatefulsets,
		"offDaemonsets":            offDaemonsets,
		"offCronjobs":              offCronjobs,
		"imageStringCacheTTLSec":   imageStringCacheTTLSec,
		"imageCheckIntervalSec":    imageCheckIntervalSec,
		"controllerWatchKey":       controllerWatchKey,
		"controllerWatchNamespace": controllerWatchNamespace,
		"slackWebhookConfigured":   slackWebhook != "", // webhook URL은 시크릿이므로 로그에 남기지 않는다
		"slackMsgPrefix":           slackMsgPrefix,
		"bindAddress":              bindAddress,
		"clusterName":              clusterName,
		"slackEventConfigured":     slackEventWebhook != "",
		"slackEventWindowSec":      slackEventWindowSec,
	})
}

// startHealthServer /healthz, /readyz, /metrics HTTP 서버를 기동하고 ctx 취소 시 정리한다.
func startHealthServer(ctx context.Context, ready func() bool) {

	if bindAddress == "" {
		return
	}

	server := &http.Server{Addr: bindAddress, Handler: health.NewMux(ready)}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("health server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

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

func newLogger(ctx context.Context) *logger.Logger {
	logger := logger.NewLogger()

	if slackWebhook != "" {
		logger.WithSlack(ctx, slackWebhook, slackMsgPrefix)
	}
	return logger
}

func main() {

	// SIGINT/SIGTERM 수신 시 ctx가 취소되어 전체 컴포넌트가 정리된다
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	clientset := newClientset()
	logger := newLogger(ctx)

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

	// 이미지 변경 구조화 이벤트: 윈도우 동안 모아 Slack에 요약 1건으로 전송
	var aggregator *events.Aggregator
	if slackEventWebhook != "" {
		notifier := events.NewSlackNotifier(slackEventWebhook, clusterName, slackEventWindowSec)
		aggregator = events.NewAggregator(slackEventWindowSec, notifier.Notify)
		opt.ImagePatchedSink = aggregator
	}

	synced := watcher.Run(ctx, opt, clientset, &wg, logger)

	// 모든 informer 캐시가 동기화되면 ready
	startHealthServer(ctx, func() bool {
		for _, hasSynced := range synced {
			if !hasSynced() {
				return false
			}
		}
		return true
	})

	<-ctx.Done()
	klog.Warning("Signal received, stopping")
	wg.Wait()

	// 컨트롤러 정지 후 잔여 이벤트 요약을 동기 전송해 종료 시 알림 유실을 막는다
	if aggregator != nil {
		aggregator.Flush()
	}

}
