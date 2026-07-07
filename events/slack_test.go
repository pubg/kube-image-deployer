package events

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const ecr = "123.dkr.ecr.us-east-1.amazonaws.com"

func sampleSummary() Summary {
	apiTo := ecr + "/api@sha256:" + strings.Repeat("b", 64)
	return BuildSummary([]ImagePatched{
		// 같은 이미지가 세 workload에 적용
		{Resource: "deployments", Namespace: "default", Name: "api", Changes: []ContainerImageChange{
			{Container: "app", WatchURL: ecr + "/api", WatchTag: "release-*", From: ecr + "/api:release-*", To: apiTo, ToTag: "release-1102"},
		}},
		{Resource: "deployments", Namespace: "default", Name: "worker", Changes: []ContainerImageChange{
			{Container: "app", WatchURL: ecr + "/api", WatchTag: "release-*", From: ecr + "/api@sha256:old", To: apiTo, ToTag: "release-1102"},
			{Container: "sidecar", WatchURL: ecr + "/api", WatchTag: "release-*", From: ecr + "/api@sha256:old", To: apiTo, ToTag: "release-1102"},
		}},
		{Resource: "cronjobs", Namespace: "batch", Name: "sync", Changes: []ContainerImageChange{
			{Container: "app", WatchURL: ecr + "/api", WatchTag: "release-*", From: ecr + "/api@sha256:old", To: apiTo, ToTag: "release-1102"},
		}},
		{Resource: "deployments", Namespace: "default", Name: "myapp", Changes: []ContainerImageChange{
			{Container: "app", WatchURL: "busybox", WatchTag: "1.34.*", From: "busybox:1.34.*",
				To: "busybox@sha256:" + strings.Repeat("a", 64), ToTag: "1.34.1"},
		}},
		{Resource: "statefulsets", Namespace: "infra", Name: "redis", Changes: []ContainerImageChange{
			{Container: "redis", WatchURL: "redis", WatchTag: "7.*", From: "redis:7.*",
				To: "redis@sha256:" + strings.Repeat("f", 64), ToTag: "7.4.1"},
		}},
	})
}

func sectionTexts(t *testing.T, blocks []map[string]any) string {
	t.Helper()
	texts := make([]string, 0)
	for _, block := range blocks {
		if block["type"] != "section" {
			continue
		}
		texts = append(texts, block["text"].(map[string]any)["text"].(string))
	}
	return strings.Join(texts, "\n===\n")
}

// 이미지 중심 레이아웃: 이미지(감시 url:tag → 해석 태그 + digest)를 헤더로,
// 아래에 적용된 namespace · kind/name (containers) 나열.
func TestBuildBlocksFormat(t *testing.T) {

	at := time.Date(2026, 7, 6, 7, 12, 0, 0, time.UTC)
	blocks := BuildBlocks(sampleSummary(), "prod-us-east", 20, at)

	if blocks[0]["type"] != "header" {
		t.Fatalf("first block must be header: %v", blocks[0])
	}
	if headerJSON, _ := json.Marshal(blocks[0]); !strings.Contains(string(headerJSON), "prod-us-east") {
		t.Fatalf("header must contain cluster name: %s", headerJSON)
	}

	contextJSON, _ := json.Marshal(blocks[1])
	if blocks[1]["type"] != "context" ||
		!strings.Contains(string(contextJSON), "20s") ||
		!strings.Contains(string(contextJSON), "이미지 *3*종") ||
		!strings.Contains(string(contextJSON), "workload *5*개") ||
		!strings.Contains(string(contextJSON), "컨테이너 *6*개") {
		t.Fatalf("context must contain image/workload/container counts: %s", contextJSON)
	}

	all := sectionTexts(t, blocks)

	// 이미지 헤더: 호스트 제거 + 해석 태그 + digest 12자
	if !strings.Contains(all, "`/api:release-*` → `release-1102 (@sha256:"+strings.Repeat("b", 12)+")`") {
		t.Errorf("api image header missing: %s", all)
	}
	if !strings.Contains(all, "`busybox:1.34.*` → `1.34.1 (@sha256:"+strings.Repeat("a", 12)+")`") {
		t.Errorf("busybox image header missing: %s", all)
	}

	// 같은 이미지의 적용 대상들이 한 묶음 아래에 나열 (namespace, kind, name 정렬)
	if !strings.Contains(all,
		"\n └ batch · cronjob/sync (app)\n └ default · deployment/api (app)\n └ default · deployment/worker (app, sidecar)") {
		t.Errorf("api targets must be listed under one image group: %s", all)
	}

	// digest 12자 축약
	if strings.Contains(all, strings.Repeat("b", 13)) {
		t.Errorf("digest must be truncated to 12 chars: %s", all)
	}

	// 이미지 묶음 사이 빈 줄
	if !strings.Contains(all, ")`\n\n`") && !strings.Contains(all, ")\n\n`") {
		t.Errorf("image groups must be separated by blank line: %s", all)
	}

}

// 해석된 태그가 없으면 digest만 표기한다 (하위 호환).
func TestDisplayToWithoutTag(t *testing.T) {

	if got := displayTo("busybox@sha256:"+strings.Repeat("e", 64), ""); got != "@sha256:"+strings.Repeat("e", 12) {
		t.Errorf("without tag: %q", got)
	}
	if got := displayTo("busybox@sha256:"+strings.Repeat("e", 64), "dev-111"); got != "dev-111 (@sha256:"+strings.Repeat("e", 12)+")" {
		t.Errorf("with tag: %q", got)
	}

}

// 내용이 섹션 텍스트 한도를 넘으면 여러 섹션으로 나뉘고, 내용 유실이 없어야 한다.
func TestSectionChunking(t *testing.T) {

	// 이미지 하나가 200개 workload에 적용된 극단 케이스
	list := make([]ImagePatched, 0, 200)
	for i := range 200 {
		list = append(list, ImagePatched{
			Resource: "deployments", Namespace: "big", Name: fmt.Sprintf("workload-%03d", i),
			Changes: []ContainerImageChange{{Container: "app", WatchURL: ecr + "/service", WatchTag: "release-*",
				From: ecr + "/service:release-*", To: ecr + "/service@sha256:" + strings.Repeat("f", 64), ToTag: "release-999"}},
		})
	}

	blocks := BuildBlocks(BuildSummary(list), "prod", 20, time.Unix(0, 0).UTC())

	sections := 0
	for _, block := range blocks {
		if block["type"] != "section" {
			continue
		}
		sections++
		text := block["text"].(map[string]any)["text"].(string)
		if len(text) > maxSectionTextLength {
			t.Fatalf("section text exceeds limit: %d > %d", len(text), maxSectionTextLength)
		}
	}
	if sections < 2 {
		t.Fatalf("200 targets must span multiple sections, got %d", sections)
	}

	all := sectionTexts(t, blocks)
	for i := range 200 {
		if !strings.Contains(all, fmt.Sprintf("workload-%03d", i)) {
			t.Fatalf("workload-%03d missing from output", i)
		}
	}

}

// 블록 수가 Slack 한도(50)를 넘으면 여러 메시지로 분할되어야 한다.
func TestPaginateBlocks(t *testing.T) {

	blocks := make([]map[string]any, 0, 120)
	for range 120 {
		blocks = append(blocks, mrkdwnSection("x"))
	}

	pages := paginateBlocks(blocks)

	if len(pages) != 3 {
		t.Fatalf("120 blocks must span 3 messages, pages=%d", len(pages))
	}

	total := 0
	for _, page := range pages {
		if len(page) > maxBlocksPerMessage {
			t.Fatalf("page exceeds slack block limit: %d", len(page))
		}
		total += len(page)
	}
	if total != len(blocks) {
		t.Fatalf("pagination must not drop blocks: %d != %d", total, len(blocks))
	}

}

// SlackNotifier는 페이지별로 웹훅에 전송해야 한다.
func TestSlackNotifierSend(t *testing.T) {

	received := make([]string, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = append(received, string(body))
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	notifier := NewSlackNotifier(server.URL, "prod-us-east", 20)
	notifier.Notify(sampleSummary())

	if len(received) != 1 {
		t.Fatalf("expected 1 webhook call, got %d", len(received))
	}
	if !strings.Contains(received[0], `"blocks"`) || !strings.Contains(received[0], "prod-us-east") {
		t.Errorf("unexpected payload: %s", received[0])
	}

}

// HTTP 상태 코드가 200이 아니면 body가 "ok"여도 에러여야 한다.
func TestSlackNotifierNon200Status(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	notifier := NewSlackNotifier(server.URL, "prod", 20)

	if err := notifier.send([]map[string]any{mrkdwnSection("x")}); err == nil {
		t.Fatal("expected error on non-200 status")
	} else if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should include status code: %v", err)
	}

}
