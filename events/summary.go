package events

import (
	"sort"
	"strings"
)

// ImageTarget 이미지가 적용된 workload 하나
type ImageTarget struct {
	Namespace  string
	Kind       string // deployment | statefulset | daemonset | cronjob (단수형)
	Name       string
	Containers []string // 컨테이너 이름 목록. init 컨테이너는 "name (init)" 표기
}

// ImageSummary 이미지 하나의 변경 요약 — 감시 대상(url:tag)과 적용된 digest, 적용 위치들
type ImageSummary struct {
	WatchURL       string // 감시 대상 이미지 URL (annotation의 url)
	WatchTag       string // 감시 대상 태그 패턴 (예: release-*)
	To             string // 적용된 imageString (url@sha256:...)
	ToTag          string // digest가 취득된 원본 태그 (예: release-1102)
	Targets        []ImageTarget
	ContainerCount int
}

// Summary 윈도우 내 전체 변경 요약 (이미지 중심 그룹핑)
type Summary struct {
	Images          []ImageSummary
	TotalImages     int
	TotalWorkloads  int
	TotalContainers int
}

// BuildSummary 윈도우 내 이벤트들을 이미지(감시 대상 + 적용 digest) 기준으로 그룹핑한다.
// 같은 컨테이너가 여러 번 패치된 경우 최종 To로 병합하고,
// 병합 결과 From == To(윈도우 내 원복)인 변경은 제외한다.
func BuildSummary(list []ImagePatched) Summary {

	type workloadKey struct {
		namespace string
		resource  string
		name      string
	}
	type containerKey struct {
		container string
		isInit    bool
	}

	// 1. 컨테이너 단위 dedupe (최초 From / 최종 To)
	merged := make(map[workloadKey]map[containerKey]*ContainerImageChange)

	for _, event := range list {
		wk := workloadKey{namespace: event.Namespace, resource: event.Resource, name: event.Name}
		if merged[wk] == nil {
			merged[wk] = make(map[containerKey]*ContainerImageChange)
		}
		for _, change := range event.Changes {
			ck := containerKey{container: change.Container, isInit: change.IsInitContainer}
			if exists, ok := merged[wk][ck]; ok {
				exists.To = change.To // 최종 To로 갱신, From은 최초 값 유지
				exists.ToTag = change.ToTag
				exists.WatchURL = change.WatchURL // 윈도우 중 annotation이 바뀐 경우 최신 감시 대상 기준으로 그룹핑
				exists.WatchTag = change.WatchTag
			} else {
				c := change
				merged[wk][ck] = &c
			}
		}
	}

	// 2. 이미지(감시 대상 + 적용 digest) 기준 그룹핑
	type imageGroupKey struct {
		watchURL string
		watchTag string
		to       string
		toTag    string
	}

	groups := make(map[imageGroupKey]map[workloadKey][]ContainerImageChange)
	workloadSet := make(map[workloadKey]bool)
	totalContainers := 0

	for wk, containers := range merged {
		for _, change := range containers {
			if change.From == change.To { // 윈도우 내 원복 → 순변화 없음
				continue
			}
			gk := imageGroupKey{watchURL: change.WatchURL, watchTag: change.WatchTag, to: change.To, toTag: change.ToTag}
			if groups[gk] == nil {
				groups[gk] = make(map[workloadKey][]ContainerImageChange)
			}
			groups[gk][wk] = append(groups[gk][wk], *change)
			workloadSet[wk] = true
			totalContainers++
		}
	}

	// 3. Summary 구성 및 정렬
	summary := Summary{
		TotalImages:     len(groups),
		TotalWorkloads:  len(workloadSet),
		TotalContainers: totalContainers,
	}

	for gk, targets := range groups {
		image := ImageSummary{
			WatchURL: gk.watchURL,
			WatchTag: gk.watchTag,
			To:       gk.to,
			ToTag:    gk.toTag,
		}

		for wk, changes := range targets {
			sort.Slice(changes, func(i, j int) bool {
				a, b := changes[i], changes[j]
				if a.IsInitContainer != b.IsInitContainer {
					return !a.IsInitContainer // 일반 컨테이너 먼저
				}
				return a.Container < b.Container
			})

			containers := make([]string, 0, len(changes))
			for _, change := range changes {
				name := change.Container
				if change.IsInitContainer {
					name += " (init)"
				}
				containers = append(containers, name)
			}

			image.Targets = append(image.Targets, ImageTarget{
				Namespace:  wk.namespace,
				Kind:       strings.TrimSuffix(wk.resource, "s"),
				Name:       wk.name,
				Containers: containers,
			})
			image.ContainerCount += len(containers)
		}

		sort.Slice(image.Targets, func(i, j int) bool {
			a, b := image.Targets[i], image.Targets[j]
			if a.Namespace != b.Namespace {
				return a.Namespace < b.Namespace
			}
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
			}
			return a.Name < b.Name
		})

		summary.Images = append(summary.Images, image)
	}

	sort.Slice(summary.Images, func(i, j int) bool {
		a, b := summary.Images[i], summary.Images[j]
		if a.WatchURL != b.WatchURL {
			return a.WatchURL < b.WatchURL
		}
		if a.WatchTag != b.WatchTag {
			return a.WatchTag < b.WatchTag
		}
		return a.ToTag < b.ToTag
	})

	return summary

}
