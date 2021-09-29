package controller

import (
	"encoding/json"
	"fmt"

	"github.com/pubg/kube-image-deployer/util"
	v1 "k8s.io/api/core/v1"
)

type patch struct {
	key           string
	containerName string
	url           string
	tag           string
	imageString   string
}

type imageUpdateNotify struct {
	url         string
	tag         string
	imageString string
}

type Container struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type ImageStrategicPatch struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers     []Container `json:"containers,omitempty"`
				InitContainers []Container `json:"initContainers,omitempty"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

func (c *Controller) getPatchMapByUpdates(updates []imageUpdateNotify) map[string][]patch {
	// key 별로 kubernetes patch apply를 위한 리스트를 생성
	patchMap := make(map[string][]patch)

	for _, update := range updates {
		c.logger.Infof("[%s] OnUpdateImageString %s, %s, %s", c.resource, update.url, update.tag, update.imageString)

		c.syncedImagesMutex.RLock()
		defer c.syncedImagesMutex.RUnlock()

		for image := range c.syncedImages {

			if image.url != update.url || image.tag != update.tag {
				continue
			}

			if patchMap[image.key] == nil {
				patchMap[image.key] = make([]patch, 0)
			}

			patchMap[image.key] = append(patchMap[image.key], patch{
				key:           image.key,
				containerName: image.containerName,
				url:           update.url,
				tag:           update.tag,
				imageString:   update.imageString,
			})

		}
	}

	return patchMap
}

func (c *Controller) applyPatchList(key string, patchList []patch) error {

	imageStrategicPatch := ImageStrategicPatch{}
	Containers := make([]Container, 0)
	InitContainers := make([]Container, 0)
	namespace, name := util.GetNamespaceNameByKey(key)

	if namespace == "" || name == "" {
		return fmt.Errorf("[%s] OnUpdateImageString patch invalid key=%s", c.resource, key)
	}

	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("[%s] OnUpdateImageString patch error key=%s err=%s", c.resource, key, err)
	}

	for _, patch := range patchList {
		c.logger.Infof("[%s] OnUpdateImageString patch %+v", c.resource, patch)

		if !exists { // 삭제된 리소스인 경우 무시
			return fmt.Errorf("[%s] OnUpdateImageString patch not exists key=%s", c.resource, key)
		} else { // 이미지 업데이트

			var currentContainer v1.Container
			isInitContainer := false

			if find, err := util.GetContainerByName(obj, patch.containerName); err == nil {
				currentContainer = find // container에서 찾음
			} else if find, err := util.GetInitContainerByName(obj, patch.containerName); err == nil {
				currentContainer = find // initContainer에서 찾음
				isInitContainer = true
			} else { // 컨테이너 이름을 찾지 못함
				return fmt.Errorf("[%s] OnUpdateImageString patch error key=%s, containerName=%s, err=%s", c.resource, key, patch.containerName, err)
			}

			// 이미지 변경 체크
			if currentContainer.Image != patch.imageString {
				container := Container{
					Name:  patch.containerName,
					Image: patch.imageString,
				}
				if isInitContainer {
					InitContainers = append(InitContainers, container)
				} else {
					Containers = append(Containers, container)
				}
			}
		}
	}

	if len(Containers) == 0 && len(InitContainers) == 0 { // 변경된 이미지가 없는 경우 무시
		c.logger.Infof("[%s] OnUpdateImageString patch containers not changed %+v", c.resource, patchList)
		return nil
	}

	imageStrategicPatch.Spec.Template.Spec.Containers = Containers
	imageStrategicPatch.Spec.Template.Spec.InitContainers = InitContainers
	patchString, err := json.Marshal(imageStrategicPatch)

	if err != nil {
		return fmt.Errorf("[%s] OnUpdateImageString patch marshal error %+v, err=%s", c.resource, patchList, err)
	}

	if err := c.applyStrategicMergePatch(namespace, name, patchString); err != nil {
		return fmt.Errorf("[%s] OnUpdateImageString patch apply error namespace=%s, name=%s, patchString=%s, err=%s", c.resource, namespace, name, patchString, err)
	}

	c.logger.Warningf("[%s] OnUpdateImageString patch apply success namespace=%s, name=%s, patchString=%s", c.resource, namespace, name, patchString)
	return nil

}

// OnUpdateImageString is a controller function that is called when an image hash is updated
func (c *Controller) OnUpdateImageString(url, tag, platformString, imageString string) {

	notify := imageUpdateNotify{
		url:         url,
		tag:         tag,
		imageString: imageString,
	}

	c.imageUpdateNotifyListMutex.Lock()
	c.imageUpdateNotifyList = append(c.imageUpdateNotifyList, notify)
	c.imageUpdateNotifyListMutex.Unlock()

}

// patchUpdateNotifyList 일정 시간마다 ImageUpdateNotifyList에 쌓인 업데이트 정보를 Kubernetes에 Apply하는 트리거
func (c *Controller) patchUpdateNotifyList() {

	c.imageUpdateNotifyListMutex.Lock()
	updates := c.imageUpdateNotifyList                     // list 복사
	c.imageUpdateNotifyList = make([]imageUpdateNotify, 0) // list 비움
	c.imageUpdateNotifyListMutex.Unlock()

	if len(updates) == 0 {
		return
	}

	patchMap := c.getPatchMapByUpdates(updates)

	for key, patchList := range patchMap {
		if err := c.applyPatchList(key, patchList); err != nil {
			c.logger.Errorf(err.Error()) // just logging
		}
	}

}
