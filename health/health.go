// Package health liveness/readiness probe와 Prometheus 메트릭 HTTP 핸들러.
package health

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewMux /healthz(liveness), /readyz(readiness), /metrics(Prometheus)를 제공하는 mux를 생성한다.
// ready는 informer 캐시 동기화 여부 등 준비 상태를 반환하는 함수다.
func NewMux(ready func() bool) *http.ServeMux {

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if ready == nil || !ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.Handle("/metrics", promhttp.Handler())

	return mux

}
