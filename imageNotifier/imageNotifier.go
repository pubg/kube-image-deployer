package imageNotifier

import (
	"sync"
	"time"

	"github.com/pubg/kube-image-deployer/interfaces"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

type ImageNotifierId struct {
	controller interfaces.IController
	url        string
	tag        string
}

type ImageNotifier struct {
	list   map[ImageNotifierId]*ImageUpdateNotify
	mutex  sync.RWMutex
	stopCh chan struct{}

	remoteRegistry interfaces.IRemoteRegistry
}

func NewImageNotifier(stopCh chan struct{}, remoteRegistry interfaces.IRemoteRegistry, imageCheckIntervalSec uint) *ImageNotifier {
	imageNotifier := &ImageNotifier{
		list:           make(map[ImageNotifierId]*ImageUpdateNotify),
		mutex:          sync.RWMutex{},
		stopCh:         stopCh,
		remoteRegistry: remoteRegistry,
	}

	go wait.Until(imageNotifier.checkAllImageNotifyList, time.Second*time.Duration(imageCheckIntervalSec), stopCh)

	return imageNotifier
}

// RegistImage regist to imageNotifier
func (r *ImageNotifier) RegistImage(controller interfaces.IController, url, tag string) {

	notifyId := ImageNotifierId{controller: controller, url: url, tag: tag}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	existsImageUpdateNotify, ok := r.list[notifyId]

	if ok { // 중복 키?
		existsImageUpdateNotify.addReferenceCount()
		return
	}

	// 신규
	klog.Infof("[%s] RegistImage %s:%s\n", controller.GetReresourceName(), url, tag)

	imageUpdateNotify := NewImageUpdateNotify(url, tag, "", controller)

	r.list[notifyId] = imageUpdateNotify
}

// UnregistImage unregist from imageNotifier
func (r *ImageNotifier) UnregistImage(controller interfaces.IController, url, tag string) {

	notifyId := ImageNotifierId{controller: controller, url: url, tag: tag}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	existsImageUpdateNotify, ok := r.list[notifyId]

	if !ok { // ??
		klog.Errorf("[%s] UnregistImage existsImageUpdateNotify notfound url=%s, tag=%s", controller.GetReresourceName(), url, tag)
		return
	}

	referenceCount := existsImageUpdateNotify.subReferenceCount()

	if referenceCount <= 0 { // 이미지를 참조하는 대상이 더이상 없으면 삭제
		delete(r.list, notifyId)
		klog.Infof("[%s] UnregistImage %s:%s\n", controller.GetReresourceName(), url, tag)
	}

}

func (r *ImageNotifier) checkImageUpdate(image checkImage) {
	imageString, err := r.remoteRegistry.GetImageString(image.url, image.tag)
	if err != nil {
		klog.Errorf("[%s] checkImageUpdate %s:%s err=%s\n", image.controller.GetReresourceName(), image.url, image.tag, err)
		return
	}

	image.controller.OnUpdateImageString(image.url, image.tag, imageString)
}

type checkImage struct {
	controller interfaces.IController
	url        string
	tag        string
}

func (r *ImageNotifier) checkAllImageNotifyList() {

	// dump all checkList
	checkList := func() []checkImage {
		list := make([]checkImage, 0)
		r.mutex.RLock()
		for _, imageUpdateNotify := range r.list {
			if imageUpdateNotify != nil {
				list = append(list, checkImage{
					controller: imageUpdateNotify.controller,
					url:        imageUpdateNotify.url,
					tag:        imageUpdateNotify.tag,
				})
			}
		}
		r.mutex.RUnlock()
		return list
	}()

	for _, check := range checkList {
		r.checkImageUpdate(check)
	}
}
