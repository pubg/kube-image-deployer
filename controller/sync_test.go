package controller

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/pubg/kube-image-deployer/events"
	"github.com/pubg/kube-image-deployer/interfaces"
	"github.com/pubg/kube-image-deployer/logger"
	appV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func newTestController() *Controller {
	return NewController(ControllerOpt{
		Resource:           "deployments",
		ControllerWatchKey: "kube-image-deployer",
		Logger:             logger.NewLogger(),
	})
}

func newDeploymentWithAnnotations(annotations map[string]string) *appV1.Deployment {
	return &appV1.Deployment{
		ObjectMeta: metaV1.ObjectMeta{
			Name:        "test",
			Namespace:   "default",
			Annotations: annotations,
		},
	}
}

func TestGetImagesFromCurrentWorkload(t *testing.T) {

	c := newTestController()
	key := "default/test"

	obj := newDeploymentWithAnnotations(map[string]string{
		"kube-image-deployer/app":      "busybox:1.34.*",                           // 일반 이미지
		"kube-image-deployer/app-port": "registry.example.com:5000/app:1.0",        // 포트 포함 레지스트리
		"kube-image-deployer/app-hash": "busybox@sha256:aaaa",                      // digest 고정은 무시
		"kube-image-deployer/app-none": "busybox",                                  // 태그 없음은 무시
		"other-prefix/app":             "busybox:1.34",                             // watchKey 불일치는 무시
		"kube-image-deployer/app-ecr":  "123.dkr.ecr.us-east-1.amazonaws.com/x:v1", // ECR
		"kube-image-deployer/app-npt":  "registry.example.com:5000/app",            // 포트만 있고 태그 없음은 무시
	})

	images := c.getImagesFromCurrentWorkload(obj, key)

	expected := map[Image]bool{
		{key: key, containerName: "app", url: "busybox", tag: "1.34.*"}:                               true,
		{key: key, containerName: "app-port", url: "registry.example.com:5000/app", tag: "1.0"}:       true,
		{key: key, containerName: "app-ecr", url: "123.dkr.ecr.us-east-1.amazonaws.com/x", tag: "v1"}: true,
	}

	if len(images) != len(expected) {
		t.Fatalf("expected %d images, got %d: %+v", len(expected), len(images), images)
	}

	for image := range expected {
		if !images[image] {
			t.Errorf("expected image not found: %+v (got %+v)", image, images)
		}
	}

}

type notifierEvent struct {
	action string // "regist" | "unregist"
	url    string
	tag    string
}

type fakeNotifier struct {
	mutex  sync.Mutex
	events []notifierEvent
}

func (f *fakeNotifier) RegistImage(listener interfaces.IImageListener, url, tag, platformString string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.events = append(f.events, notifierEvent{action: "regist", url: url, tag: tag})
}

func (f *fakeNotifier) UnregistImage(listener interfaces.IImageListener, url, tag, platformString string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.events = append(f.events, notifierEvent{action: "unregist", url: url, tag: tag})
}

func (f *fakeNotifier) snapshot() []notifierEvent {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return append([]notifierEvent{}, f.events...)
}

// syncKey가 반환된 시점에는 imageNotifier 등록/해제가 완료되어 있어야 한다.
// fire-and-forget goroutine으로 디스패치하면 순서가 보장되지 않아
// 빠른 변경 시 Unregist가 Regist보다 먼저 실행되어 참조 카운트가 꼬일 수 있다.
func TestSyncKeyRegistUnregistSynchronous(t *testing.T) {

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	notifier := &fakeNotifier{}

	c := NewController(ControllerOpt{
		Resource:           "deployments",
		ControllerWatchKey: "kube-image-deployer",
		Logger:             logger.NewLogger(),
		Indexer:            indexer,
		ImageNotifier:      notifier,
	})

	key := "default/test"

	// 1. workload 생성 → 등록
	obj := newDeploymentWithAnnotations(map[string]string{"kube-image-deployer/app": "busybox:1.34"})
	if err := indexer.Add(obj); err != nil {
		t.Fatal(err)
	}
	if err := c.syncKey(key); err != nil {
		t.Fatal(err)
	}

	if evts := notifier.snapshot(); len(evts) != 1 || evts[0] != (notifierEvent{"regist", "busybox", "1.34"}) {
		t.Fatalf("regist must complete before syncKey returns, got %+v", evts)
	}

	// 2. 태그 변경 → 새 이미지 등록 + 이전 이미지 해제
	obj2 := newDeploymentWithAnnotations(map[string]string{"kube-image-deployer/app": "busybox:1.35"})
	if err := indexer.Update(obj2); err != nil {
		t.Fatal(err)
	}
	if err := c.syncKey(key); err != nil {
		t.Fatal(err)
	}

	evts := notifier.snapshot()
	if len(evts) != 3 ||
		evts[1] != (notifierEvent{"regist", "busybox", "1.35"}) ||
		evts[2] != (notifierEvent{"unregist", "busybox", "1.34"}) {
		t.Fatalf("expected regist(1.35) then unregist(1.34), got %+v", evts)
	}

	// 3. workload 삭제 → 해제
	if err := indexer.Delete(obj2); err != nil {
		t.Fatal(err)
	}
	if err := c.syncKey(key); err != nil {
		t.Fatal(err)
	}

	evts = notifier.snapshot()
	if len(evts) != 4 || evts[3] != (notifierEvent{"unregist", "busybox", "1.35"}) {
		t.Fatalf("expected unregist(1.35) after delete, got %+v", evts)
	}

}

// patchList의 일부 컨테이너가 workload에 존재하지 않아도 (예: 컨테이너 이름 변경)
// 나머지 유효한 컨테이너의 패치는 진행되어야 한다.
func TestApplyPatchListPartialContainerMissing(t *testing.T) {

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	applied := make([]string, 0)
	apply := func(namespace, name string, data []byte) error {
		applied = append(applied, namespace+"/"+name+" "+string(data))
		return nil
	}

	c := NewController(ControllerOpt{
		Resource:                 "deployments",
		ControllerWatchKey:       "kube-image-deployer",
		Logger:                   logger.NewLogger(),
		Indexer:                  indexer,
		ImageNotifier:            &fakeNotifier{},
		ApplyStrategicMergePatch: apply,
	})

	obj := newDeploymentWithAnnotations(map[string]string{"kube-image-deployer/app": "busybox:1.34"})
	obj.Spec.Template.Spec.Containers = []coreV1.Container{{Name: "app", Image: "busybox:1.34"}}

	if err := indexer.Add(obj); err != nil {
		t.Fatal(err)
	}

	key := "default/test"
	patchList := []patch{
		{key: key, containerName: "ghost", url: "busybox", tag: "1.34", imageString: "busybox@sha256:new"}, // 존재하지 않는 컨테이너
		{key: key, containerName: "app", url: "busybox", tag: "1.34", imageString: "busybox@sha256:new"},   // 존재하는 컨테이너
	}

	if err := c.applyPatchList(key, patchList); err != nil {
		t.Logf("applyPatchList returned err (acceptable): %v", err)
	}

	if len(applied) != 1 {
		t.Fatalf("valid container must be patched even if another is missing, applied=%v", applied)
	}
	if !strings.Contains(applied[0], `"name":"app"`) || !strings.Contains(applied[0], "busybox@sha256:new") {
		t.Fatalf("unexpected patch: %s", applied[0])
	}

}

// newReconcileTestController workqueue를 포함한 reconcile 테스트용 컨트롤러를 생성한다.
func newReconcileTestController(apply ApplyStrategicMergePatch) (*Controller, cache.Indexer) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	c := NewController(ControllerOpt{
		Resource:                 "deployments",
		ControllerWatchKey:       "kube-image-deployer",
		Logger:                   logger.NewLogger(),
		Indexer:                  indexer,
		ImageNotifier:            &fakeNotifier{},
		Queue:                    workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
		ApplyStrategicMergePatch: apply,
	})
	return c, indexer
}

func newWatchedDeployment(image string) *appV1.Deployment {
	obj := newDeploymentWithAnnotations(map[string]string{"kube-image-deployer/app": "busybox:1.34"})
	obj.Spec.Template.Spec.Containers = []coreV1.Container{{Name: "app", Image: image}}
	return obj
}

// digest 변경 통지는 해당 이미지를 사용하는 workload key만 workqueue에 넣어야 한다.
func TestOnUpdateImageStringEnqueuesAffectedWorkloads(t *testing.T) {

	c, indexer := newReconcileTestController(func(namespace, name string, data []byte) error { return nil })

	if err := indexer.Add(newWatchedDeployment("busybox:1.34")); err != nil {
		t.Fatal(err)
	}
	if err := c.syncKey("default/test"); err != nil { // 이미지 등록
		t.Fatal(err)
	}

	c.OnUpdateImageString("busybox", "1.34", "", interfaces.ResolvedImage{ImageString: "busybox@sha256:new", Tag: "1.34"})
	if c.queue.Len() != 1 {
		t.Fatalf("notify must enqueue the affected workload key, queue len=%d", c.queue.Len())
	}

	c.OnUpdateImageString("other", "9", "", interfaces.ResolvedImage{ImageString: "other@sha256:x", Tag: "9"}) // 미등록 이미지
	if c.queue.Len() != 1 {
		t.Fatalf("unrelated image must not enqueue, queue len=%d", c.queue.Len())
	}

}

// 통지 후 reconcile(syncKey)이 desired imageString으로 패치를 적용해야 하고,
// 이미 반영된 상태에서는 패치를 반복하지 않아야 한다.
func TestReconcileAppliesPatchOnNotify(t *testing.T) {

	applied := make([]string, 0)
	c, indexer := newReconcileTestController(func(namespace, name string, data []byte) error {
		applied = append(applied, string(data))
		return nil
	})

	key := "default/test"

	if err := indexer.Add(newWatchedDeployment("busybox:1.34")); err != nil {
		t.Fatal(err)
	}
	if err := c.syncKey(key); err != nil {
		t.Fatal(err)
	}

	if len(applied) != 0 {
		t.Fatalf("no patch expected before any notify, applied=%v", applied)
	}

	c.OnUpdateImageString("busybox", "1.34", "", interfaces.ResolvedImage{ImageString: "busybox@sha256:new", Tag: "1.34"})

	if !c.processNextItem() { // queue에서 key를 꺼내 reconcile
		t.Fatal("processNextItem returned false")
	}

	if len(applied) != 1 || !strings.Contains(applied[0], "busybox@sha256:new") {
		t.Fatalf("patch with desired imageString expected, applied=%v", applied)
	}

	// 패치가 반영된 workload 상태에서는 재-sync가 패치를 반복하지 않아야 한다
	if err := indexer.Update(newWatchedDeployment("busybox@sha256:new")); err != nil {
		t.Fatal(err)
	}
	if err := c.syncKey(key); err != nil {
		t.Fatal(err)
	}

	if len(applied) != 1 {
		t.Fatalf("no patch expected when already up to date, applied=%v", applied)
	}

}

// 패치 실패 시 workqueue가 백오프와 함께 재시도해야 한다.
func TestReconcileRetriesOnPatchFailure(t *testing.T) {

	calls := 0
	c, indexer := newReconcileTestController(func(namespace, name string, data []byte) error {
		calls++
		if calls == 1 {
			return fmt.Errorf("transient api error")
		}
		return nil
	})

	if err := indexer.Add(newWatchedDeployment("busybox:1.34")); err != nil {
		t.Fatal(err)
	}
	if err := c.syncKey("default/test"); err != nil {
		t.Fatal(err)
	}

	c.OnUpdateImageString("busybox", "1.34", "", interfaces.ResolvedImage{ImageString: "busybox@sha256:new", Tag: "1.34"})

	c.processNextItem() // 1차: 실패 → requeue
	c.processNextItem() // 2차: 백오프 후 재시도 → 성공

	if calls != 2 {
		t.Fatalf("patch must be retried after failure, calls=%d", calls)
	}
	if c.queue.Len() != 0 {
		t.Fatalf("queue must be empty after successful retry, len=%d", c.queue.Len())
	}

}

type fakeEventSink struct {
	mutex  sync.Mutex
	events []events.ImagePatched
}

func (f *fakeEventSink) OnImagePatched(event events.ImagePatched) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.events = append(f.events, event)
}

func (f *fakeEventSink) count() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return len(f.events)
}

// 패치 적용 성공 시 실제 변경된 컨테이너의 from→to를 담은 ImagePatched 이벤트가 발행되어야 하고,
// 변화가 없거나 패치가 실패한 경우에는 발행되지 않아야 한다.
func TestApplyPatchListEmitsImagePatched(t *testing.T) {

	applyErr := error(nil)
	c, indexer := newReconcileTestController(func(namespace, name string, data []byte) error {
		return applyErr
	})

	sink := &fakeEventSink{}
	c.imagePatchedSink = sink

	key := "default/test"
	if err := indexer.Add(newWatchedDeployment("busybox:1.34")); err != nil {
		t.Fatal(err)
	}
	if err := c.syncKey(key); err != nil {
		t.Fatal(err)
	}

	patchList := []patch{{key: key, containerName: "app", url: "busybox", tag: "1.34", imageString: "busybox@sha256:new", resolvedTag: "1.34"}}

	// 1. 패치 성공 → 이벤트 발행
	if err := c.applyPatchList(key, patchList); err != nil {
		t.Fatal(err)
	}
	if sink.count() != 1 {
		t.Fatalf("successful patch must emit 1 event, got %d", sink.count())
	}

	event := sink.events[0]
	if event.Resource != "deployments" || event.Namespace != "default" || event.Name != "test" {
		t.Fatalf("unexpected event identity: %+v", event)
	}
	if len(event.Changes) != 1 ||
		event.Changes[0].Container != "app" ||
		event.Changes[0].WatchURL != "busybox" ||
		event.Changes[0].WatchTag != "1.34" ||
		event.Changes[0].From != "busybox:1.34" ||
		event.Changes[0].To != "busybox@sha256:new" ||
		event.Changes[0].ToTag != "1.34" {
		t.Fatalf("unexpected changes: %+v", event.Changes)
	}

	// 2. 이미 반영된 상태 (no-op) → 발행 없음
	if err := indexer.Update(newWatchedDeployment("busybox@sha256:new")); err != nil {
		t.Fatal(err)
	}
	if err := c.applyPatchList(key, patchList); err != nil {
		t.Fatal(err)
	}
	if sink.count() != 1 {
		t.Fatalf("no-op patch must not emit, got %d", sink.count())
	}

	// 3. 패치 실패 → 발행 없음
	if err := indexer.Update(newWatchedDeployment("busybox:1.34")); err != nil {
		t.Fatal(err)
	}
	applyErr = fmt.Errorf("api error")
	if err := c.applyPatchList(key, patchList); err == nil {
		t.Fatal("expected patch error")
	}
	if sink.count() != 1 {
		t.Fatalf("failed patch must not emit, got %d", sink.count())
	}

}

// 감시 중인 workload가 하나도 없는 이미지의 통지는 desired 상태에 저장하지 않아야 한다.
// (in-flight 체크와 unregist가 경합하면 정리된 desiredImages에 고아 엔트리가 남아,
// 같은 url:tag 재등록 시 낡은 digest로 즉시 패치될 수 있다)
func TestOnUpdateImageStringIgnoredWhenUnwatched(t *testing.T) {

	c, _ := newReconcileTestController(func(namespace, name string, data []byte) error { return nil })

	c.OnUpdateImageString("ghost", "1", "", interfaces.ResolvedImage{ImageString: "ghost@sha256:stale", Tag: "1"})

	if c.queue.Len() != 0 {
		t.Fatalf("unwatched notify must not enqueue, len=%d", c.queue.Len())
	}

	c.desiredImagesMutex.RLock()
	_, ok := c.desiredImages[imageKey{url: "ghost", tag: "1"}]
	c.desiredImagesMutex.RUnlock()

	if ok {
		t.Fatal("unwatched notify must not store desired image (orphan entry)")
	}

}

type countingLogger struct {
	mutex  sync.Mutex
	errors int
	debugs int
}

func (l *countingLogger) Infof(format string, args ...any)    {}
func (l *countingLogger) Warningf(format string, args ...any) {}
func (l *countingLogger) Errorf(format string, args ...any) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.errors++
}
func (l *countingLogger) Debugf(format string, args ...any) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.debugs++
}

// 반복 실패는 처음 몇 회만 Error(Slack 미러)로 알리고, 이후에는 klog 전용으로 강등되어야 한다.
// (영구 실패가 백오프 상한 간격으로 Slack을 계속 울리는 것을 방지. 재시도는 무기한 계속)
func TestHandleErrDemotesToDebugAfterRetries(t *testing.T) {

	logger := &countingLogger{}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	c := NewController(ControllerOpt{
		Resource:           "deployments",
		ControllerWatchKey: "kube-image-deployer",
		Logger:             logger,
		Indexer:            indexer,
		ImageNotifier:      &fakeNotifier{},
		Queue:              workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
	})

	for range 7 {
		c.handleErr(fmt.Errorf("permanent failure"), "default/test")
	}

	if logger.errors != 5 || logger.debugs != 2 {
		t.Fatalf("expected 5 Errorf + 2 Debugf, got %d/%d", logger.errors, logger.debugs)
	}

}
