package util

import (
	"strings"
	"testing"

	appV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func podSpecFixture() coreV1.PodSpec {
	return coreV1.PodSpec{
		Containers:     []coreV1.Container{{Name: "app", Image: "busybox:1"}},
		InitContainers: []coreV1.Container{{Name: "init", Image: "busybox:2"}},
	}
}

// workloadFixtures 지원하는 4개 workload 타입의 픽스처를 생성한다.
func workloadFixtures() map[string]any {

	annotations := map[string]string{"kube-image-deployer/app": "busybox:1"}
	meta := metaV1.ObjectMeta{Name: "test", Namespace: "default", Annotations: annotations}

	deployment := &appV1.Deployment{ObjectMeta: meta}
	deployment.Spec.Template.Spec = podSpecFixture()

	statefulset := &appV1.StatefulSet{ObjectMeta: meta}
	statefulset.Spec.Template.Spec = podSpecFixture()

	daemonset := &appV1.DaemonSet{ObjectMeta: meta}
	daemonset.Spec.Template.Spec = podSpecFixture()

	cronjob := &batchV1.CronJob{ObjectMeta: meta}
	cronjob.Spec.JobTemplate.Spec.Template.Spec = podSpecFixture()

	return map[string]any{
		"deployment":  deployment,
		"statefulset": statefulset,
		"daemonset":   daemonset,
		"cronjob":     cronjob,
	}
}

// 지원하는 모든 workload 타입에서 접근자가 동일하게 동작해야 한다.
func TestWorkloadAccessors(t *testing.T) {

	for name, obj := range workloadFixtures() {

		if annotations, err := GetAnnotations(obj); err != nil {
			t.Errorf("[%s] GetAnnotations err: %v", name, err)
		} else if annotations["kube-image-deployer/app"] != "busybox:1" {
			t.Errorf("[%s] unexpected annotations: %v", name, annotations)
		}

		if containers, err := GetContainers(obj); err != nil || len(containers) != 1 || containers[0].Name != "app" {
			t.Errorf("[%s] GetContainers: %v, err=%v", name, containers, err)
		}

		if initContainers, err := GetInitContainers(obj); err != nil || len(initContainers) != 1 || initContainers[0].Name != "init" {
			t.Errorf("[%s] GetInitContainers: %v, err=%v", name, initContainers, err)
		}

		if container, err := GetContainerByName(obj, "app"); err != nil || container.Image != "busybox:1" {
			t.Errorf("[%s] GetContainerByName: %+v, err=%v", name, container, err)
		}

		if container, err := GetInitContainerByName(obj, "init"); err != nil || container.Image != "busybox:2" {
			t.Errorf("[%s] GetInitContainerByName: %+v, err=%v", name, container, err)
		}

		if _, err := GetContainerByName(obj, "ghost"); err == nil {
			t.Errorf("[%s] GetContainerByName must fail for unknown container", name)
		}

	}

	// 지원하지 않는 타입은 에러
	if _, err := GetAnnotations(struct{}{}); err == nil {
		t.Error("GetAnnotations must fail for unsupported type")
	}
	if _, err := GetContainers("bogus"); err == nil {
		t.Error("GetContainers must fail for unsupported type")
	}

}

// CronJob은 jobTemplate 하위 경로로, 나머지는 spec.template 경로로 패치가 생성되어야 한다.
func TestGetImageStrategicPatchJson(t *testing.T) {

	fixtures := workloadFixtures()
	containers := []Container{{Name: "app", Image: "busybox@sha256:x"}}
	initContainers := []Container{{Name: "init", Image: "busybox@sha256:y"}}

	for _, name := range []string{"deployment", "statefulset", "daemonset"} {
		patchJson, err := GetImageStrategicPatchJson(fixtures[name], containers, initContainers)
		if err != nil {
			t.Fatalf("[%s] err: %v", name, err)
		}
		s := string(patchJson)
		if strings.Contains(s, "jobTemplate") {
			t.Errorf("[%s] must not contain jobTemplate: %s", name, s)
		}
		if !strings.Contains(s, `"containers":[{"name":"app","image":"busybox@sha256:x"}]`) ||
			!strings.Contains(s, `"initContainers":[{"name":"init","image":"busybox@sha256:y"}]`) {
			t.Errorf("[%s] unexpected patch: %s", name, s)
		}
	}

	patchJson, err := GetImageStrategicPatchJson(fixtures["cronjob"], containers, nil)
	if err != nil {
		t.Fatalf("[cronjob] err: %v", err)
	}
	if s := string(patchJson); !strings.Contains(s, `"jobTemplate"`) {
		t.Errorf("[cronjob] patch must nest under jobTemplate: %s", s)
	}

}
