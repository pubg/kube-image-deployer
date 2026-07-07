package logger

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestSlack(url string) *Slack {
	return &Slack{
		webhookUrl: url,
		msgPrefix:  "[test]",
		httpClient: &http.Client{Timeout: time.Second},
	}
}

func decodeBlocks(t *testing.T, body string) SlackRequestBody {
	t.Helper()
	var payload SlackRequestBody
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("invalid payload: %v (%s)", err, body)
	}
	return payload
}

// 메시지는 rich_text > rich_text_preformatted 블록 구조로 전송되어야 한다.
// (plain text로 개별 전송하면 Slack에서 메시지가 여러 개로 쪼개진다)
func TestSlackSendRichTextFormat(t *testing.T) {

	var received string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	s := newTestSlack(server.URL)

	if err := s.send(buildBlocks([]string{"line one", "line two"})); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	payload := decodeBlocks(t, received)

	if len(payload.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(payload.Blocks))
	}

	block := payload.Blocks[0]
	if block.Type != "rich_text" {
		t.Errorf("block type: got %q, want rich_text", block.Type)
	}
	if len(block.Elements) != 1 || block.Elements[0].Type != "rich_text_preformatted" {
		t.Fatalf("expected single rich_text_preformatted element, got %+v", block.Elements)
	}
	if text := block.Elements[0].Elements[0].Text; text != "line one\nline two" {
		t.Errorf("lines must be joined with newline in one block, got %q", text)
	}

}

func TestSlackSendNonOkResponse(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid_payload"))
	}))
	defer server.Close()

	s := newTestSlack(server.URL)

	if err := s.send(buildBlocks([]string{"x"})); err == nil {
		t.Fatal("expected error on non-ok response")
	} else if !strings.Contains(err.Error(), "invalid_payload") {
		t.Errorf("error should include response body: %v", err)
	}

}

// 한 블록의 텍스트는 Slack 제한(3000자)을 넘지 않도록 줄 단위로 청킹되어야 한다.
func TestSlackBuildBlocksChunking(t *testing.T) {

	// 1000자 줄 7개 → 한 블록에 2줄(2001자)씩 → 4개 블록
	lines := make([]string, 7)
	for i := range lines {
		lines[i] = strings.Repeat("a", 1000)
	}

	blocks := buildBlocks(lines)

	if len(blocks) != 4 {
		t.Fatalf("expected 4 blocks, got %d", len(blocks))
	}

	total := ""
	for _, b := range blocks {
		text := b.Elements[0].Elements[0].Text
		if len(text) > maxBlockTextLength {
			t.Errorf("block text exceeds limit: %d > %d", len(text), maxBlockTextLength)
		}
		if total != "" {
			total += "\n"
		}
		total += text
	}

	if total != strings.Join(lines, "\n") {
		t.Error("chunked blocks must preserve full content")
	}

}

// 단일 줄이 제한을 넘으면 강제로 분할되어야 한다 (전송 실패 방지).
func TestSlackBuildBlocksOversizedLine(t *testing.T) {

	blocks := buildBlocks([]string{strings.Repeat("b", maxBlockTextLength*2+100)})

	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks for oversized line, got %d", len(blocks))
	}
	for _, b := range blocks {
		if len(b.Elements[0].Elements[0].Text) > maxBlockTextLength {
			t.Error("block text exceeds limit after split")
		}
	}

}

// pushMessage로 쌓인 메시지는 하나의 요청으로 배칭 전송되어야 한다.
func TestSlackBatchDelivery(t *testing.T) {

	received := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- string(body)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewSlack(ctx, server.URL, "[test]")
	s.InfoDepth(0, "first message")
	s.ErrorDepth(0, "second message")

	select {
	case body := <-received:
		payload := decodeBlocks(t, body)
		if len(payload.Blocks) != 1 {
			t.Fatalf("batched messages must be merged into one block, got %d blocks", len(payload.Blocks))
		}
		text := payload.Blocks[0].Elements[0].Elements[0].Text
		if !strings.Contains(text, "first message") || !strings.Contains(text, "second message") {
			t.Errorf("batched block must contain both messages: %s", text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no slack delivery within 5s")
	}

}

// HTTP 상태 코드가 200이 아니면 body가 "ok"여도 에러여야 한다.
func TestSlackSendNon200Status(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	s := newTestSlack(server.URL)

	if err := s.send(buildBlocks([]string{"x"})); err == nil {
		t.Fatal("expected error on non-200 status")
	} else if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should include status code: %v", err)
	}

}
