package main

import (
	"bytes"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pubg/kube-image-deployer/logger"
	"k8s.io/klog/v2"
)

// captureStderr fn 실행 동안 os.Stderr(klog 기본 출력 대상)에 기록된 내용을 반환한다.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()
	klog.Flush()
	w.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// CLI 호출자는 stderr 출력 유무로 실패를 감지하기도 하므로, 성공 경로의 정보성
// klog가 stderr로 새어나가면 정상 결과가 실패로 오판된다. v0.2.0에서 ECR 토큰
// 취득 성공 Infof(remoteRegistry/docker/ecr.go)가 그대로 노출되어 실제 발생한 문제.
func TestConfigureLoggingSilencesInfoLogs(t *testing.T) {

	configureLogging(false)

	got := captureStderr(t, func() {
		// ecr.go의 토큰 취득 성공 로그와 동일한 경로(logger → klog)
		logger.NewLogger().Infof("ECRAuthenticator GetAuthorizationToken success url=%s", "123.dkr.ecr.us-east-1.amazonaws.com")
	})

	if got != "" {
		t.Fatalf("info log must not leak to stderr: %q", got)
	}

}

// -verbose를 주면 라이브러리 로그를 stderr에서 확인할 수 있어야 한다 (디버깅용).
func TestConfigureLoggingVerboseKeepsKlog(t *testing.T) {

	klog.LogToStderr(true) // 앞선 테스트가 억제한 상태를 기본값으로 복원

	configureLogging(true)

	got := captureStderr(t, func() {
		logger.NewLogger().Infof("verbose probe")
	})

	if !strings.Contains(got, "verbose probe") {
		t.Fatalf("verbose mode must keep klog on stderr: %q", got)
	}

}

// 성공 실행의 계약: exit 0, stdout에 digest 한 줄, stderr는 완전 침묵.
func TestRunResolvesDigestWithSilentStderr(t *testing.T) {

	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	url := host + "/test/app"

	// 두 태그를 push해 와일드카드가 최고 버전(1.0.2)으로 해석되는지 본다
	older, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	newer, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	refOlder, err := name.ParseReference(url + ":1.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(refOlder, older); err != nil {
		t.Fatal(err)
	}
	refNewer, err := name.ParseReference(url + ":1.0.2")
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(refNewer, newer); err != nil {
		t.Fatal(err)
	}

	digest, err := newer.Digest()
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	var code int

	klogOut := captureStderr(t, func() {
		code = run([]string{"--image", url, "--tag", "1.0.*"}, &stdout, &stderr)
	})

	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}
	if want := url + "@" + digest.String() + "\n"; stdout.String() != want {
		t.Fatalf("stdout %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 || klogOut != "" {
		t.Fatalf("stderr must be silent on success: run=%q klog=%q", stderr.String(), klogOut)
	}

}

// 실패는 exit code 1과 stderr의 error 메시지로 전달되어야 한다.
func TestRunFailureWritesErrorToStderr(t *testing.T) {

	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--image", host + "/none/missing", "--tag", "1.0.0"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "error:") {
		t.Fatalf("stderr must contain error message: %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout must be empty on failure: %q", stdout.String())
	}

}
