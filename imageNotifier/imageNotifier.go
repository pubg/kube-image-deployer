package imageNotifier

import (
	"context"
	"sync"
	"time"

	"github.com/pubg/kube-image-deployer/interfaces"
	l "github.com/pubg/kube-image-deployer/logger"
	"github.com/pubg/kube-image-deployer/metrics"
	"k8s.io/apimachinery/pkg/util/wait"
)

type ImageNotifierId struct {
	listener       interfaces.IImageListener
	url            string
	tag            string
	platformString string // "", "linux/amd64", "linux/386", "linux/arm32", "linux/arm32v7" ...
}

type ImageNotifier struct {
	list  map[ImageNotifierId]*ImageUpdateNotify
	mutex sync.RWMutex

	remoteRegistry interfaces.IRemoteRegistry
	logger         interfaces.ILogger
}

func NewImageNotifier(ctx context.Context, remoteRegistry interfaces.IRemoteRegistry, imageCheckIntervalSec uint) *ImageNotifier {
	r := &ImageNotifier{
		list:           make(map[ImageNotifierId]*ImageUpdateNotify),
		mutex:          sync.RWMutex{},
		remoteRegistry: remoteRegistry,
		logger:         l.NewLogger(),
	}

	go wait.Until(r.checkAllImageNotifyList, time.Second*time.Duration(imageCheckIntervalSec), ctx.Done())

	return r
}

func (r *ImageNotifier) WithLogger(logger interfaces.ILogger) *ImageNotifier {
	r.logger = logger
	return r
}

// RegistImage regist to imageNotifier
func (r *ImageNotifier) RegistImage(listener interfaces.IImageListener, url, tag, platformString string) {

	notifyId := ImageNotifierId{listener: listener, url: url, tag: tag, platformString: platformString}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	existsImageUpdateNotify, ok := r.list[notifyId]

	if ok { // 중복 키?
		existsImageUpdateNotify.addReferenceCount()
		return
	}

	// 신규
	r.logger.Debugf("[%s] RegistImage %s:%s\n", listener.GetResourceName(), url, tag)

	imageUpdateNotify := NewImageUpdateNotify(url, tag, platformString, listener)

	r.list[notifyId] = imageUpdateNotify
	metrics.WatchedImages.Set(float64(len(r.list)))
}

// UnregistImage unregist from imageNotifier
func (r *ImageNotifier) UnregistImage(listener interfaces.IImageListener, url, tag, platformString string) {

	notifyId := ImageNotifierId{listener: listener, url: url, tag: tag, platformString: platformString}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	existsImageUpdateNotify, ok := r.list[notifyId]

	if !ok { // ??
		r.logger.Errorf("[%s] UnregistImage existsImageUpdateNotify notfound url=%s, tag=%s", listener.GetResourceName(), url, tag)
		return
	}

	referenceCount := existsImageUpdateNotify.subReferenceCount()

	if referenceCount <= 0 { // 이미지를 참조하는 대상이 더이상 없으면 삭제
		delete(r.list, notifyId)
		metrics.WatchedImages.Set(float64(len(r.list)))
		r.logger.Debugf("[%s] UnregistImage %s:%s\n", listener.GetResourceName(), url, tag)
	}

}

// checkImageUpdate 이미지의 현재 imageString(digest)을 조회하고,
// 직전 값과 달라진 경우에만 controller에 통지한다.
// 패치 실패 재시도는 controller의 workqueue가 담당하므로 매 주기 통지할 필요가 없다.
func (r *ImageNotifier) checkImageUpdate(entry *ImageUpdateNotify) {
	image, err := r.remoteRegistry.GetImageString(entry.url, entry.tag, entry.platform)
	if err != nil {
		metrics.ImageCheckTotal.WithLabelValues("error").Inc()
		r.logger.Errorf("[%s] checkImageUpdate %s:%s err=%s\n", entry.listener.GetResourceName(), entry.url, entry.tag, err)
		return
	}

	metrics.ImageCheckTotal.WithLabelValues("success").Inc()

	if entry.lastImageString == image.ImageString { // 변화 없음 (digest 기준)
		return
	}

	metrics.ImageUpdateNotifyTotal.Inc()
	entry.lastImageString = image.ImageString
	entry.listener.OnUpdateImageString(entry.url, entry.tag, entry.platform, image)
}

// maxConcurrentImageChecks 이미지 체크 동시 실행 상한.
// 느린 레지스트리 하나가 전체 체크 주기를 지연시키지 않도록 병렬로 수행하되,
// 레지스트리에 과도한 동시 요청을 보내지 않도록 제한한다.
const maxConcurrentImageChecks = 8

func (r *ImageNotifier) checkAllImageNotifyList() {

	// dump all checkList
	checkList := func() []*ImageUpdateNotify {
		list := make([]*ImageUpdateNotify, 0)
		r.mutex.RLock()
		for _, imageUpdateNotify := range r.list {
			if imageUpdateNotify != nil {
				list = append(list, imageUpdateNotify)
			}
		}
		r.mutex.RUnlock()
		return list
	}()

	semaphore := make(chan struct{}, maxConcurrentImageChecks)
	var wg sync.WaitGroup

	for _, entry := range checkList {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(entry *ImageUpdateNotify) {
			defer wg.Done()
			defer func() { <-semaphore }()
			r.checkImageUpdate(entry)
		}(entry)
	}

	wg.Wait()
}
