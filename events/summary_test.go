package events

import (
	"strings"
	"testing"
)

// 이벤트들이 이미지(감시 대상 + 적용 digest) 기준으로 그룹핑되어야 한다.
// 같은 이미지가 여러 workload에 적용되면 하나의 그룹 아래에 대상들이 나열된다.
func TestBuildSummaryGroupsByImage(t *testing.T) {

	apiTo := "123.dkr.ecr.us-east-1.amazonaws.com/api@sha256:" + strings.Repeat("b", 64)

	list := []ImagePatched{
		// 같은 이미지(api:release-*)가 세 workload에 적용 → 한 그룹
		{Resource: "deployments", Namespace: "default", Name: "api", Changes: []ContainerImageChange{
			{Container: "app", WatchURL: "123.dkr.ecr.us-east-1.amazonaws.com/api", WatchTag: "release-*",
				From: "123.dkr.ecr.us-east-1.amazonaws.com/api:release-*", To: apiTo, ToTag: "release-1102"},
		}},
		{Resource: "deployments", Namespace: "default", Name: "worker", Changes: []ContainerImageChange{
			{Container: "app", WatchURL: "123.dkr.ecr.us-east-1.amazonaws.com/api", WatchTag: "release-*",
				From: "123.dkr.ecr.us-east-1.amazonaws.com/api@sha256:old", To: apiTo, ToTag: "release-1102"},
			{Container: "sidecar", WatchURL: "123.dkr.ecr.us-east-1.amazonaws.com/api", WatchTag: "release-*",
				From: "123.dkr.ecr.us-east-1.amazonaws.com/api@sha256:old", To: apiTo, ToTag: "release-1102"},
		}},
		{Resource: "cronjobs", Namespace: "batch", Name: "sync", Changes: []ContainerImageChange{
			{Container: "app", WatchURL: "123.dkr.ecr.us-east-1.amazonaws.com/api", WatchTag: "release-*",
				From: "123.dkr.ecr.us-east-1.amazonaws.com/api@sha256:old", To: apiTo, ToTag: "release-1102"},
		}},
		// 다른 이미지 — 같은 컨테이너 재패치는 최종 To로 병합(dedupe)
		{Resource: "deployments", Namespace: "default", Name: "myapp", Changes: []ContainerImageChange{
			{Container: "app", WatchURL: "busybox", WatchTag: "1.34.*",
				From: "busybox:1.34.*", To: "busybox@sha256:old", ToTag: "1.34.0"},
		}},
		{Resource: "deployments", Namespace: "default", Name: "myapp", Changes: []ContainerImageChange{
			{Container: "app", WatchURL: "busybox", WatchTag: "1.34.*",
				From: "busybox@sha256:old", To: "busybox@sha256:" + strings.Repeat("a", 64), ToTag: "1.34.1"},
		}},
		// 윈도우 내 원복 (From == To) → 제외
		{Resource: "deployments", Namespace: "default", Name: "rollback", Changes: []ContainerImageChange{
			{Container: "c", WatchURL: "r", WatchTag: "1", From: "r@sha256:x", To: "r@sha256:x", ToTag: "1"},
		}},
	}

	s := BuildSummary(list)

	if s.TotalImages != 2 || s.TotalWorkloads != 4 || s.TotalContainers != 5 {
		t.Fatalf("totals: images=%d workloads=%d containers=%d, want 2/4/5", s.TotalImages, s.TotalWorkloads, s.TotalContainers)
	}

	if len(s.Images) != 2 {
		t.Fatalf("expected 2 image groups: %+v", s.Images)
	}

	// 정렬: WatchURL 기준 — 123.dkr... < busybox
	api := s.Images[0]
	if api.WatchURL != "123.dkr.ecr.us-east-1.amazonaws.com/api" || api.WatchTag != "release-*" ||
		api.To != apiTo || api.ToTag != "release-1102" {
		t.Fatalf("unexpected api group: %+v", api)
	}
	if len(api.Targets) != 3 || api.ContainerCount != 4 {
		t.Fatalf("api targets: %+v (containers=%d)", api.Targets, api.ContainerCount)
	}

	// 대상 정렬: namespace, kind, name — batch/cronjob/sync → default/deployment/api → default/deployment/worker
	if api.Targets[0].Namespace != "batch" || api.Targets[0].Kind != "cronjob" || api.Targets[0].Name != "sync" {
		t.Fatalf("targets must be sorted: %+v", api.Targets)
	}

	// 같은 workload의 컨테이너는 하나의 target에 모인다
	worker := api.Targets[2]
	if worker.Name != "worker" || len(worker.Containers) != 2 ||
		worker.Containers[0] != "app" || worker.Containers[1] != "sidecar" {
		t.Fatalf("worker containers must be merged: %+v", worker)
	}

	// dedupe: 재패치는 최종 To/ToTag
	busybox := s.Images[1]
	if busybox.ToTag != "1.34.1" || busybox.To != "busybox@sha256:"+strings.Repeat("a", 64) {
		t.Fatalf("dedupe must keep last To: %+v", busybox)
	}

}

// init 컨테이너는 이름에 (init) 표기가 붙는다.
func TestBuildSummaryInitContainerMark(t *testing.T) {

	s := BuildSummary([]ImagePatched{
		{Resource: "deployments", Namespace: "default", Name: "payment", Changes: []ContainerImageChange{
			{Container: "migrate", IsInitContainer: true, WatchURL: "m", WatchTag: "v*",
				From: "m:v*", To: "m@sha256:" + strings.Repeat("c", 64), ToTag: "v3"},
		}},
	})

	if len(s.Images) != 1 || len(s.Images[0].Targets) != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if got := s.Images[0].Targets[0].Containers[0]; got != "migrate (init)" {
		t.Fatalf("init container must be marked: %q", got)
	}

}
