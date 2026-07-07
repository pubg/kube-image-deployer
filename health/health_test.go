package health

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pubg/kube-image-deployer/metrics"
)

func get(t *testing.T, server *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// /healthz는 항상 200, /readyz는 ready 함수 결과에 따라 200/503을 반환해야 한다.
func TestHealthEndpoints(t *testing.T) {

	ready := false

	server := httptest.NewServer(NewMux(func() bool { return ready }))
	defer server.Close()

	if code, body := get(t, server, "/healthz"); code != http.StatusOK || body != "ok" {
		t.Errorf("/healthz: code=%d body=%q", code, body)
	}

	if code, _ := get(t, server, "/readyz"); code != http.StatusServiceUnavailable {
		t.Errorf("/readyz must be 503 when not ready, code=%d", code)
	}

	ready = true

	if code, body := get(t, server, "/readyz"); code != http.StatusOK || body != "ok" {
		t.Errorf("/readyz must be 200 when ready, code=%d body=%q", code, body)
	}

}

// /metrics는 등록된 kube_image_deployer 메트릭을 노출해야 한다.
func TestMetricsEndpoint(t *testing.T) {

	metrics.ImageCheckTotal.WithLabelValues("success").Inc() // 메트릭이 최소 1회 등록되도록

	server := httptest.NewServer(NewMux(func() bool { return true }))
	defer server.Close()

	code, body := get(t, server, "/metrics")

	if code != http.StatusOK {
		t.Fatalf("/metrics: code=%d", code)
	}
	if !strings.Contains(body, "kube_image_deployer_image_check_total") {
		t.Error("/metrics must expose kube_image_deployer_image_check_total")
	}

}
