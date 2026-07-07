package imageNotifier

import (
	"sync/atomic"

	"github.com/pubg/kube-image-deployer/interfaces"
)

type ImageUpdateNotify struct {
	url            string
	tag            string
	platform       string
	listener       interfaces.IImageListener
	referenceCount int32

	// lastImageString 마지막으로 통지한 imageString.
	// 체크 사이클은 겹치지 않으므로(WaitGroup) 체크 goroutine에서만 접근한다.
	lastImageString string
}

func NewImageUpdateNotify(url, tag, platform string, listener interfaces.IImageListener) *ImageUpdateNotify {
	return &ImageUpdateNotify{
		url:            url,
		tag:            tag,
		platform:       platform,
		listener:       listener,
		referenceCount: 1,
	}
}

func (u *ImageUpdateNotify) addReferenceCount() int32 {
	return atomic.AddInt32(&u.referenceCount, 1)
}

func (u *ImageUpdateNotify) subReferenceCount() int32 {
	return atomic.AddInt32(&u.referenceCount, -1)
}
