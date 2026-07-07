package interfaces

import "context"

// ResolvedImage 레지스트리에서 해석된 이미지 정보.
type ResolvedImage struct {
	ImageString string // 패치에 사용할 문자열 (url@sha256:...)
	Tag         string // digest가 취득된 원본 태그 (예: dev-* 감시 → dev-111). 정확 태그 감시면 그 태그 자체.
}

// IImageListener 이미지 digest 변경 통지를 수신하는 최소 인터페이스.
// imageNotifier는 controller 전체가 아니라 이 인터페이스에만 의존한다.
type IImageListener interface {
	OnUpdateImageString(url, tag, platformString string, image ResolvedImage)
	GetResourceName() string
}

type IController interface {
	IImageListener
	Run(ctx context.Context, workers int)
}

type IImageNotifier interface {
	RegistImage(listener IImageListener, url, tag, platformString string)
	UnregistImage(listener IImageListener, url, tag, platformString string)
}

type IRemoteRegistry interface {
	GetImageString(url, tag, platformString string) (ResolvedImage, error)
}

type ILogger interface {
	Infof(format string, args ...any)
	Errorf(format string, args ...any)
	Warningf(format string, args ...any)
	// Debugf 상세 흐름 로그 — klog에만 기록하고 Slack으로는 미러하지 않는다.
	// 이미지 변경처럼 배포 웨이브에서 대량 발생하는 로그에 사용한다 (알림은 구조화 이벤트가 담당).
	Debugf(format string, args ...any)
}
