package events

import (
	"sync"
	"time"
)

// Aggregator 이미지 패치 이벤트를 윈도우 동안 모아 하나의 Summary로 발행한다.
// 배포 웨이브에서 수십 개의 workload가 회전해도 알림은 윈도우당 1건으로 뭉친다.
type Aggregator struct {
	windowSec uint
	publish   func(Summary)

	mutex   sync.Mutex
	buffer  []ImagePatched
	pending bool // 윈도우 타이머 동작 중 여부
}

// NewAggregator windowSec 동안 이벤트를 모아 publish로 발행하는 Aggregator를 생성한다.
// windowSec이 0이면 집계 없이 즉시 발송한다.
// 종료 시 잔여 버퍼 유실을 막으려면 소유자(main)가 컨트롤러 정지 후 Flush()를 호출해야 한다.
func NewAggregator(windowSec uint, publish func(Summary)) *Aggregator {
	return &Aggregator{
		windowSec: windowSec,
		publish:   publish,
	}
}

// OnImagePatched 이벤트를 버퍼에 추가한다. 버퍼가 비어있던 상태에서 첫 이벤트가
// 도착하면 윈도우 타이머를 시작하고, windowSec이 0이면 즉시(비동기) 발송한다.
// 호출자는 블로킹되지 않는다.
func (a *Aggregator) OnImagePatched(event ImagePatched) {

	a.mutex.Lock()
	a.buffer = append(a.buffer, event)
	startTimer := !a.pending && a.windowSec > 0
	if startTimer {
		a.pending = true
	}
	a.mutex.Unlock()

	if a.windowSec == 0 { // 집계 없이 즉시 발송 (호출자를 HTTP 전송으로 블로킹하지 않는다)
		go a.Flush()
		return
	}

	if startTimer {
		time.AfterFunc(time.Duration(a.windowSec)*time.Second, a.Flush)
	}

}

// Flush 버퍼의 이벤트를 요약해 발행한다. 버퍼가 비어있으면 아무것도 하지 않는다.
func (a *Aggregator) Flush() {

	a.mutex.Lock()
	buffer := a.buffer
	a.buffer = nil
	a.pending = false
	a.mutex.Unlock()

	if len(buffer) == 0 {
		return
	}

	a.publish(BuildSummary(buffer))

}
