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
	} else { // workload 생성 / 변경
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
	}
	return nil
}

// getImagesFromAnnotationValue key에서 kubernetes workload의 변경 감지 대상 이미지 추출
// kube-image-deployer/containerName=image:tag
func (c *Controller) getImagesFromCurrentWorkload(obj interface{}, key string) (images map[Image]bool) {

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

		arr := strings.Split(annotationValue, ":")
		if len(arr) == 2 {
			image := Image{
				key:           key,
				containerName: containerName,
				url:           arr[0],
				tag:           arr[1],
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
		c.logger.Infof("[%s] registImage image=%+v\n", c.resource, image)
		c.syncedImages[image] = true
		c.syncedImagesMutex.Unlock()
	}

	go c.imageNotifier.RegistImage(c, image.url, image.tag, "") // 이미지 변경 감지 등록

}

func (c *Controller) unregistImage(image Image) {

	if !c.syncedImages[image] { // 이미 제거된 이미지
		return
	}

	c.logger.Infof("[%s] unregistImage image=%+v\n", c.resource, image)

	c.syncedImagesMutex.Lock()
	delete(c.syncedImages, image)
	c.syncedImagesMutex.Unlock()

	go c.imageNotifier.UnregistImage(c, image.url, image.tag, "") // 이미지 변경 감지 해제

}
