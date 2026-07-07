// Package metrics kube-image-deployer의 Prometheus 메트릭 정의.
// /metrics 엔드포인트(health.NewMux)를 통해 노출된다.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ImageCheckTotal 원격 레지스트리 digest 체크 횟수 (result: success|error)
	ImageCheckTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kube_image_deployer_image_check_total",
		Help: "Total number of image digest checks against remote registries.",
	}, []string{"result"})

	// ImageUpdateNotifyTotal digest 변경 감지(통지) 횟수
	ImageUpdateNotifyTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kube_image_deployer_image_update_notify_total",
		Help: "Total number of detected image digest changes.",
	})

	// PatchTotal workload 패치 적용 횟수 (resource: deployments|statefulsets|..., result: success|error)
	PatchTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kube_image_deployer_patch_total",
		Help: "Total number of strategic merge patches applied to workloads.",
	}, []string{"resource", "result"})

	// WatchedImages 현재 감시 중인 이미지(listener × url:tag) 수
	WatchedImages = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "kube_image_deployer_watched_images",
		Help: "Number of image entries currently being watched.",
	})
)
