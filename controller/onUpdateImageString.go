package controller

import (
	"fmt"
	"time"

	"github.com/pubg/kube-image-deployer/events"
	"github.com/pubg/kube-image-deployer/interfaces"
	"github.com/pubg/kube-image-deployer/metrics"
	"github.com/pubg/kube-image-deployer/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type patch struct {
	key           string
	containerName string
	url           string
	tag           string
	imageString   string
	resolvedTag   string // digest가 취득된 원본 태그 (알림 표기용)
}

// imageKey desiredImages의 키. url:tag 단위로 최신 imageString을 추적한다.
type imageKey struct {
	url string
	tag string
}

// OnUpdateImageString imageNotifier가 이미지 digest 변경을 감지했을 때 호출된다.
// 최신 이미지(digest + 취득 태그)를 desired 상태로 저장하고, 해당 이미지를 사용하는
// workload key를 workqueue에 넣어 reconcile(syncKey)을 트리거한다.
// 패치 실패 시 재시도는 workqueue가 담당한다.
func (c *Controller) OnUpdateImageString(url, tag, platformString string, image interfaces.ResolvedImage) {

	c.logger.Debugf("[%s] OnUpdateImageString %s:%s -> %s (tag=%s)", c.resource, url, tag, image.ImageString, image.Tag)

	// 이 이미지를 사용하는 workload key 수집
	keys := make(map[string]bool)

	c.syncedImagesMutex.RLock()
	for synced := range c.syncedImages {
		if synced.url == url && synced.tag == tag {
			keys[synced.key] = true
		}
	}
	c.syncedImagesMutex.RUnlock()

	// 감시 중인 workload가 없으면 저장하지 않는다.
	// in-flight 체크와 unregist가 경합할 때 정리된 desiredImages에 고아 엔트리가 남아
	// 재등록 시 낡은 digest로 패치되는 창을 없앤다.
	if len(keys) == 0 {
		c.logger.Debugf("[%s] OnUpdateImageString ignored, no watched workload for %s:%s", c.resource, url, tag)
		return
	}

	c.desiredImagesMutex.Lock()
	c.desiredImages[imageKey{url: url, tag: tag}] = image
	c.desiredImagesMutex.Unlock()

	for key := range keys {
		c.queue.Add(key)
	}

}

// reconcileWorkload workload의 감시 대상 이미지들을 desired imageString과 비교해 패치를 적용한다.
// syncKey에서 호출되며, 에러 반환 시 workqueue가 백오프와 함께 재시도한다.
func (c *Controller) reconcileWorkload(key string, images map[Image]bool) error {

	patchList := make([]patch, 0)

	c.desiredImagesMutex.RLock()
	for image := range images {
		if desired, ok := c.desiredImages[imageKey{url: image.url, tag: image.tag}]; ok {
			patchList = append(patchList, patch{
				key:           key,
				containerName: image.containerName,
				url:           image.url,
				tag:           image.tag,
				imageString:   desired.ImageString,
				resolvedTag:   desired.Tag,
			})
		}
	}
	c.desiredImagesMutex.RUnlock()

	if len(patchList) == 0 { // 아직 digest가 해석되지 않은 경우
		return nil
	}

	return c.applyPatchList(key, patchList)

}

// applyPatchList workload의 현재 컨테이너 이미지와 desired imageString을 비교해
// 달라진 컨테이너만 Strategic Merge Patch로 적용한다.
func (c *Controller) applyPatchList(key string, patchList []patch) error {

	Containers := make([]util.Container, 0)
	InitContainers := make([]util.Container, 0)
	changes := make([]events.ContainerImageChange, 0)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil || namespace == "" || name == "" {
		return fmt.Errorf("[%s] applyPatchList invalid key=%s", c.resource, key)
	}

	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("[%s] applyPatchList error key=%s err=%s", c.resource, key, err)
	}

	if !exists { // 삭제된 리소스인 경우 무시
		return fmt.Errorf("[%s] applyPatchList not exists key=%s", c.resource, key)
	}

	for _, patch := range patchList {

		var currentContainer v1.Container
		isInitContainer := false

		if find, err := util.GetContainerByName(obj, patch.containerName); err == nil {
			currentContainer = find // container에서 찾음
		} else if find, err := util.GetInitContainerByName(obj, patch.containerName); err == nil {
			currentContainer = find // initContainer에서 찾음
			isInitContainer = true
		} else { // 컨테이너 이름을 찾지 못해도 나머지 컨테이너의 패치는 진행한다
			c.logger.Errorf("[%s] applyPatchList container not found, skipping key=%s, containerName=%s, err=%s", c.resource, key, patch.containerName, err)
			continue
		}

		// 이미지 변경 체크
		if currentContainer.Image != patch.imageString {
			container := util.Container{
				Name:  patch.containerName,
				Image: patch.imageString,
			}
			if isInitContainer {
				InitContainers = append(InitContainers, container)
			} else {
				Containers = append(Containers, container)
			}
			changes = append(changes, events.ContainerImageChange{
				Container:       patch.containerName,
				IsInitContainer: isInitContainer,
				WatchURL:        patch.url,
				WatchTag:        patch.tag,
				From:            currentContainer.Image,
				To:              patch.imageString,
				ToTag:           patch.resolvedTag,
			})
		}
	}

	if len(Containers) == 0 && len(InitContainers) == 0 { // 변경된 이미지가 없는 경우 무시
		return nil
	}

	patchString, err := util.GetImageStrategicPatchJson(obj, Containers, InitContainers)

	if err != nil {
		return fmt.Errorf("[%s] applyPatchList marshal error %+v, err=%s", c.resource, patchList, err)
	}

	if err := c.applyStrategicMergePatch(namespace, name, patchString); err != nil {
		metrics.PatchTotal.WithLabelValues(c.resource, "error").Inc()
		return fmt.Errorf("[%s] applyPatchList apply error namespace=%s, name=%s, patchString=%s, err=%s", c.resource, namespace, name, patchString, err)
	}

	metrics.PatchTotal.WithLabelValues(c.resource, "success").Inc()
	// 상세 내역은 klog에만 남긴다. Slack 알림은 구조화 이벤트(imagePatchedSink)가 담당한다.
	c.logger.Debugf("[%s] applyPatchList apply success namespace=%s, name=%s, patchString=%s", c.resource, namespace, name, patchString)

	if c.imagePatchedSink != nil {
		c.imagePatchedSink.OnImagePatched(events.ImagePatched{
			Resource:  c.resource,
			Namespace: namespace,
			Name:      name,
			Changes:   changes,
			Time:      time.Now(),
		})
	}

	return nil

}
