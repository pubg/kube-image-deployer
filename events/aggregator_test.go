package events

import (
	"sync"
	"testing"
	"time"
)

type capturedPublish struct {
	mutex     sync.Mutex
	summaries []Summary
}

func (c *capturedPublish) publish(s Summary) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.summaries = append(c.summaries, s)
}

func (c *capturedPublish) count() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return len(c.summaries)
}

// Flush는 버퍼의 이벤트를 요약해 1회 발행하고, 빈 버퍼에서는 발행하지 않아야 한다.
func TestAggregatorFlush(t *testing.T) {

	captured := &capturedPublish{}
	a := NewAggregator(3600, captured.publish) // 긴 윈도우 — 수동 Flush로만 발행

	a.OnImagePatched(ImagePatched{Resource: "deployments", Namespace: "default", Name: "app1",
		Changes: []ContainerImageChange{{Container: "c1", From: "a:1", To: "a@sha256:x"}}})
	a.OnImagePatched(ImagePatched{Resource: "deployments", Namespace: "default", Name: "app2",
		Changes: []ContainerImageChange{{Container: "c2", From: "b:1", To: "b@sha256:y"}}})

	a.Flush()

	if captured.count() != 1 {
		t.Fatalf("expected 1 publish, got %d", captured.count())
	}
	if s := captured.summaries[0]; s.TotalWorkloads != 2 || s.TotalContainers != 2 {
		t.Fatalf("unexpected summary: %+v", s)
	}

	a.Flush() // 빈 버퍼 → 발행 없음

	if captured.count() != 1 {
		t.Fatalf("empty flush must not publish, got %d", captured.count())
	}

}

// 첫 이벤트 도착 시 윈도우 타이머가 시작되고, 윈도우 종료 시 자동 발행되어야 한다.
func TestAggregatorWindowTimer(t *testing.T) {

	captured := &capturedPublish{}
	a := NewAggregator(1, captured.publish) // 1초 윈도우

	a.OnImagePatched(ImagePatched{Resource: "deployments", Namespace: "default", Name: "app1",
		Changes: []ContainerImageChange{{Container: "c1", From: "a:1", To: "a@sha256:x"}}})
	a.OnImagePatched(ImagePatched{Resource: "deployments", Namespace: "default", Name: "app2",
		Changes: []ContainerImageChange{{Container: "c2", From: "b:1", To: "b@sha256:y"}}})

	deadline := time.After(3 * time.Second)
	for captured.count() == 0 {
		select {
		case <-deadline:
			t.Fatal("window timer did not flush within 3s")
		case <-time.After(50 * time.Millisecond):
		}
	}

	if captured.count() != 1 {
		t.Fatalf("burst must be published as one summary, got %d", captured.count())
	}
	if s := captured.summaries[0]; s.TotalWorkloads != 2 {
		t.Fatalf("unexpected summary: %+v", s)
	}

}

// windowSec 0은 "집계 없이 즉시 발송"을 의미해야 한다.
// (기존에는 타이머가 시작되지 않아 이벤트가 버퍼에 무한히 쌓이고 알림이 발송되지 않았다)
func TestAggregatorZeroWindowFlushesImmediately(t *testing.T) {

	captured := &capturedPublish{}
	a := NewAggregator(0, captured.publish)

	a.OnImagePatched(ImagePatched{Resource: "deployments", Namespace: "default", Name: "app1",
		Changes: []ContainerImageChange{{Container: "c1", From: "a:1", To: "a@sha256:x"}}})

	deadline := time.After(2 * time.Second)
	for captured.count() == 0 {
		select {
		case <-deadline:
			t.Fatal("windowSec=0 must flush immediately, notification never sent")
		case <-time.After(20 * time.Millisecond):
		}
	}

}
