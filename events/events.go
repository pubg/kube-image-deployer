// Package events 이미지 패치 구조화 이벤트의 정의, 윈도우 집계, Slack 알림.
// 오류 로그는 기존 logger의 Slack 미러가 담당하고, 이미지 변경 알림은 이 패키지가 담당한다.
package events

import "time"

// ContainerImageChange 컨테이너 하나의 이미지 변경 내역
type ContainerImageChange struct {
	Container       string
	IsInitContainer bool
	WatchURL        string // 감시 대상 이미지 URL (annotation의 url)
	WatchTag        string // 감시 대상 태그 패턴 (예: release-*)
	From            string // 패치 직전 컨테이너 이미지
	To              string // 적용된 imageString (url@sha256:...)
	ToTag           string // digest가 취득된 원본 태그 (예: dev-* 감시 → dev-111)
}

// ImagePatched workload 하나에 패치가 적용되었을 때 발행되는 이벤트
type ImagePatched struct {
	Resource  string // deployments | statefulsets | daemonsets | cronjobs
	Namespace string
	Name      string
	Changes   []ContainerImageChange
	Time      time.Time
}

// Sink 이미지 패치 이벤트 수신 인터페이스.
// 구현체는 발행측(controller)을 블로킹하지 않아야 한다.
type Sink interface {
	OnImagePatched(event ImagePatched)
}
