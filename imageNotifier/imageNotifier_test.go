package imageNotifier

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pubg/kube-image-deployer/interfaces"
	l "github.com/pubg/kube-image-deployer/logger"
)

type registryCall struct {
	url            string
	tag            string
	platformString string
}

type fakeRegistry struct {
	mutex  sync.Mutex
	calls  []registryCall
	digest string // 비어있으면 "sha256:dummy"
}

func (f *fakeRegistry) GetImageString(url, tag, platformString string) (interfaces.ResolvedImage, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.calls = append(f.calls, registryCall{url: url, tag: tag, platformString: platformString})
	digest := f.digest
	if digest == "" {
		digest = "sha256:dummy"
	}
	// 와일드카드 감시를 흉내내 해석된 구체 태그를 함께 반환한다
	return interfaces.ResolvedImage{ImageString: url + "@" + digest, Tag: "resolved-" + tag}, nil
}

func (f *fakeRegistry) setDigest(digest string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.digest = digest
}

func (f *fakeRegistry) callCount() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return len(f.calls)
}

type controllerUpdate struct {
	url            string
	tag            string
	platformString string
	imageString    string
	toTag          string
}

type fakeController struct {
	mutex   sync.Mutex
	updates []controllerUpdate
}

func (f *fakeController) OnUpdateImageString(url, tag, platformString string, image interfaces.ResolvedImage) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.updates = append(f.updates, controllerUpdate{url: url, tag: tag, platformString: platformString, imageString: image.ImageString, toTag: image.Tag})
}

func (f *fakeController) GetResourceName() string { return "test" }

func (f *fakeController) updateCount() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return len(f.updates)
}

// newTestNotifier 백그라운드 폴링 루프 없이 notifier를 생성한다.
// (NewImageNotifier는 생성 즉시 폴링 goroutine을 시작해 테스트의 수동 체크와 경합한다)
func newTestNotifier(registry interfaces.IRemoteRegistry) *ImageNotifier {
	return &ImageNotifier{
		list:           make(map[ImageNotifierId]*ImageUpdateNotify),
		remoteRegistry: registry,
		logger:         l.NewLogger(),
	}
}

// RegistImage에 전달한 platformString은 레지스트리 조회와 컨트롤러 통지까지 유실 없이 전달되어야 한다.
func TestImageNotifierPlatformStringPropagation(t *testing.T) {

	registry := &fakeRegistry{}
	notifier := newTestNotifier(registry)

	controller := &fakeController{}
	notifier.RegistImage(controller, "busybox", "1.34", "linux/arm64")

	notifier.checkAllImageNotifyList()

	registry.mutex.Lock()
	defer registry.mutex.Unlock()

	if len(registry.calls) != 1 {
		t.Fatalf("expected 1 registry call, got %d", len(registry.calls))
	}
	if registry.calls[0].platformString != "linux/arm64" {
		t.Errorf("platformString lost in registry call: got %q, want %q", registry.calls[0].platformString, "linux/arm64")
	}

	controller.mutex.Lock()
	defer controller.mutex.Unlock()

	if len(controller.updates) != 1 {
		t.Fatalf("expected 1 controller update, got %d", len(controller.updates))
	}
	if controller.updates[0].platformString != "linux/arm64" {
		t.Errorf("platformString lost in controller notify: got %q, want %q", controller.updates[0].platformString, "linux/arm64")
	}
	if controller.updates[0].imageString != "busybox@sha256:dummy" {
		t.Errorf("unexpected imageString: %q", controller.updates[0].imageString)
	}
	if controller.updates[0].toTag != "resolved-1.34" {
		t.Errorf("resolved tag lost in notify: %q", controller.updates[0].toTag)
	}

}

// 같은 이미지를 두 번 등록하면 참조 카운트가 증가하고,
// 등록 횟수만큼 해제해야 체크 목록에서 제거되어야 한다.
func TestImageNotifierReferenceCount(t *testing.T) {

	registry := &fakeRegistry{}
	notifier := newTestNotifier(registry)

	controller := &fakeController{}

	notifier.RegistImage(controller, "busybox", "1.34", "")
	notifier.RegistImage(controller, "busybox", "1.34", "") // 중복 등록 → refcount 2

	notifier.UnregistImage(controller, "busybox", "1.34", "") // refcount 1: 아직 체크되어야 함
	notifier.checkAllImageNotifyList()

	if registry.callCount() != 1 {
		t.Fatalf("image should still be checked while refcount > 0, calls=%d", registry.callCount())
	}

	notifier.UnregistImage(controller, "busybox", "1.34", "") // refcount 0: 제거
	notifier.checkAllImageNotifyList()

	if registry.callCount() != 1 {
		t.Fatalf("image should not be checked after unregist, calls=%d", registry.callCount())
	}

}

// 모든 체크가 도착할 때까지 응답을 막는 레지스트리.
// 체크가 순차 실행되면 첫 호출이 release를 기다리며 블록되어 나머지가 도착하지 못한다.
type barrierRegistry struct {
	arrived chan struct{}
	release chan struct{}
}

func (b *barrierRegistry) GetImageString(url, tag, platformString string) (interfaces.ResolvedImage, error) {
	b.arrived <- struct{}{}
	<-b.release
	return interfaces.ResolvedImage{ImageString: url + "@sha256:dummy", Tag: tag}, nil
}

// 이미지 체크는 병렬로 수행되어야 한다.
// 순차 실행이면 느린 레지스트리 하나가 전체 체크 주기를 지연시킨다.
func TestImageNotifierParallelCheck(t *testing.T) {

	const n = 4

	registry := &barrierRegistry{
		arrived: make(chan struct{}, n),
		release: make(chan struct{}),
	}

	notifier := newTestNotifier(registry)

	controller := &fakeController{}
	for i := range n {
		notifier.RegistImage(controller, fmt.Sprintf("img%d", i), "1", "")
	}

	done := make(chan struct{})
	go func() {
		notifier.checkAllImageNotifyList()
		close(done)
	}()

	for i := range n {
		select {
		case <-registry.arrived:
		case <-time.After(3 * time.Second):
			t.Fatalf("only %d/%d checks started concurrently — checks appear to be sequential", i, n)
		}
	}

	close(registry.release)
	<-done

}

// digest가 바뀌지 않으면 통지하지 않고, 바뀐 경우에만 통지해야 한다.
// (매 주기 통지하면 controller가 매번 무의미한 reconcile을 수행한다)
func TestImageNotifierNotifyOnChangeOnly(t *testing.T) {

	registry := &fakeRegistry{}
	notifier := newTestNotifier(registry)
	controller := &fakeController{}

	notifier.RegistImage(controller, "busybox", "1.34", "")

	notifier.checkAllImageNotifyList() // 최초 해석 → 통지
	notifier.checkAllImageNotifyList() // 변화 없음 → 통지 없어야 함
	notifier.checkAllImageNotifyList() // 변화 없음 → 통지 없어야 함

	if got := controller.updateCount(); got != 1 {
		t.Fatalf("unchanged digest must not re-notify: updates=%d, want 1", got)
	}

	registry.setDigest("sha256:changed")
	notifier.checkAllImageNotifyList() // digest 변경 → 통지

	if got := controller.updateCount(); got != 2 {
		t.Fatalf("changed digest must notify: updates=%d, want 2", got)
	}

	controller.mutex.Lock()
	defer controller.mutex.Unlock()
	if controller.updates[1].imageString != "busybox@sha256:changed" {
		t.Errorf("unexpected imageString: %q", controller.updates[1].imageString)
	}

}
