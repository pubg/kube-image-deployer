package imageNotifier

import (
	"sync"

	"github.com/pubg/kube-image-deployer/interfaces"
)

type ImageUpdateNotify struct {
	url            string
	tag            string
	hash           string
	controller     interfaces.IController
	referenceCount int
	mutex          *sync.RWMutex
}

func NewImageUpdateNotify(url, tag, hash string, controller interfaces.IController) *ImageUpdateNotify {
	return &ImageUpdateNotify{
		url:            url,
		tag:            tag,
		hash:           hash,
		controller:     controller,
		referenceCount: 1,
		mutex:          &sync.RWMutex{},
	}
}

func (u *ImageUpdateNotify) Get() (url, tag, hash string) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()
	return u.url, u.tag, u.hash
}

func (u *ImageUpdateNotify) addReferenceCount() int {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	u.referenceCount++
	return u.referenceCount
}

func (u *ImageUpdateNotify) subReferenceCount() int {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	u.referenceCount--
	return u.referenceCount
}
