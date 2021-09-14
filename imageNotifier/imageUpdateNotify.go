package imageNotifier

import (
	"sync/atomic"

	"github.com/pubg/kube-image-deployer/interfaces"
)

type ImageUpdateNotify struct {
	url            string
	tag            string
	hash           string
	controller     interfaces.IController
	referenceCount int32
}

func NewImageUpdateNotify(url, tag, hash string, controller interfaces.IController) *ImageUpdateNotify {
	return &ImageUpdateNotify{
		url:            url,
		tag:            tag,
		hash:           hash,
		controller:     controller,
		referenceCount: 1,
	}
}

func (u *ImageUpdateNotify) addReferenceCount() int32 {
	return atomic.AddInt32(&u.referenceCount, 1)
}

func (u *ImageUpdateNotify) subReferenceCount() int32 {
	return atomic.AddInt32(&u.referenceCount, -1)
}
