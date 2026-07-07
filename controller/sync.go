package controller

import (
	"strings"

	"github.com/pubg/kube-image-deployer/util"
)

type Image struct {
	key           string
	containerName string
	url           string
	tag           string
}

func (c *Controller) syncKey(key string) error {

	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		c.logger.Errorf("[%s] Fetching object with key %s from store failed with %v", c.resource, key, err)
		return err
	}

	// 현재 동기화된 Image/Key/Container 정보 추출
	prevImages := c.getRegisteredImagesFromKey(key)

	if !exists { // workload 삭제됨
		for image := range prevImages {
			c.unregistImage(image)
		}
		return nil
	}

	// workload 생성 / 변경
	images := c.getImagesFromCurrentWorkload(obj, key)
	for image := range images {
		if !prevImages[image] { // 신규 추가 이미지
			c.registImage(image)
		}
	}
	for image := range prevImages { // 이전에 등록된 이미지가 삭제된 경우 해제
		if !images[image] {
			c.unregistImage(image)
		}
	}

	// desired imageString과 현재 workload 상태를 비교해 패치 적용.
	// 실패 시 에러를 반환해 workqueue가 백오프와 함께 재시도하게 한다.
	return c.reconcileWorkload(key, images)
}

// getImagesFromAnnotationValue key에서 kubernetes workload의 변경 감지 대상 이미지 추출
// kube-image-deployer/containerName=image:tag
func (c *Controller) getImagesFromCurrentWorkload(obj any, key string) (images map[Image]bool) {

	images = make(map[Image]bool)
	annotations, err := util.GetAnnotations(obj)

	if err != nil {
		c.logger.Errorf("[%s] GetAnnotations error : %v", c.resource, err)
		return
	}

	for annotationKey, annotationValue := range annotations {

		if !strings.HasPrefix(annotationKey, c.watchKey+"/") { // prefix check
			continue
		} else if strings.Contains(annotationValue, "@") { // ignore when annotationValue contains @ because it already has image hash
			continue
		}

		keys := strings.SplitN(annotationKey, "/", 2)
		if len(keys) != 2 {
			continue
		}

		containerName := keys[1]

		// registry.example.com:5000/app:1.0 처럼 URL에 포트가 포함될 수 있으므로
		// 마지막 콜론을 기준으로 url과 tag를 분리한다.
		// 콜론 뒤에 /가 있으면 태그가 아니라 포트 구분자이므로 무시한다 (태그 누락 케이스).
		if idx := strings.LastIndex(annotationValue, ":"); idx > 0 && idx < len(annotationValue)-1 && !strings.Contains(annotationValue[idx+1:], "/") {
			image := Image{
				key:           key,
				containerName: containerName,
				url:           annotationValue[:idx],
				tag:           annotationValue[idx+1:],
			}
			images[image] = true
		}

	}

	if len(images) == 0 {
		c.logger.Warningf("[%s] getImagesFromCurrentWorkload undefined or invalid annotation key=%s\n", c.resource, key)
	}

	return

}

// getRegisteredImagesFromKey key로 등록되어있는 모든 이미지 추출
func (c *Controller) getRegisteredImagesFromKey(key string) (images map[Image]bool) {

	images = make(map[Image]bool)

	c.syncedImagesMutex.RLock()
	defer c.syncedImagesMutex.RUnlock()

	for syncedImage := range c.syncedImages {
		if syncedImage.key == key {
			images[syncedImage] = true
		}
	}

	return

}

func (c *Controller) registImage(image Image) {

	c.syncedImagesMutex.Lock()

	if c.syncedImages[image] { // 이미 등록된 이미지
		c.syncedImagesMutex.Unlock()
		return
	} else {
		c.logger.Debugf("[%s] registImage image=%+v\n", c.resource, image)
		c.syncedImages[image] = true
		c.syncedImagesMutex.Unlock()
	}

	// 이미지 변경 감지 등록. 동기 호출로 등록/해제 순서를 보장한다.
	c.imageNotifier.RegistImage(c, image.url, image.tag, "")

}

func (c *Controller) unregistImage(image Image) {

	c.syncedImagesMutex.Lock()

	if !c.syncedImages[image] { // 이미 제거된 이미지
		c.syncedImagesMutex.Unlock()
		return
	}

	c.logger.Debugf("[%s] unregistImage image=%+v\n", c.resource, image)

	delete(c.syncedImages, image)

	// 같은 url:tag를 참조하는 workload가 더 없으면 desired 상태도 정리한다.
	// (남겨두면 이후 재등록 시 오래된 digest로 잘못 패치될 수 있다)
	remains := false
	for synced := range c.syncedImages {
		if synced.url == image.url && synced.tag == image.tag {
			remains = true
			break
		}
	}
	c.syncedImagesMutex.Unlock()

	if !remains {
		c.desiredImagesMutex.Lock()
		delete(c.desiredImages, imageKey{url: image.url, tag: image.tag})
		c.desiredImagesMutex.Unlock()
	}

	// 이미지 변경 감지 해제. 동기 호출로 등록/해제 순서를 보장한다.
	c.imageNotifier.UnregistImage(c, image.url, image.tag, "")

}
