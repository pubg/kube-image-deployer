package util

import (
	"encoding/json"
	"fmt"

	appV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
)

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

type ImageStrategicPatchCronJob struct {
	Spec struct {
		JobTemplate struct {
			Spec struct {
				Template struct {
					Spec struct {
						Containers     []Container `json:"containers,omitempty"`
						InitContainers []Container `json:"initContainers,omitempty"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
		} `json:"jobTemplate"`
	} `json:"spec"`
}

// workloadAdapter workload 타입별 접근 방법을 한 곳에 모은 어댑터.
// 새 workload 타입 지원은 getWorkloadAdapter에 case 하나를 추가하면 된다.
type workloadAdapter struct {
	annotations map[string]string
	podSpec     *coreV1.PodSpec
	buildPatch  func(containers, initContainers []Container) any
}

func buildDefaultPatch(containers, initContainers []Container) any {
	p := ImageStrategicPatch{}
	p.Spec.Template.Spec.Containers = containers
	p.Spec.Template.Spec.InitContainers = initContainers
	return p
}

func buildCronJobPatch(containers, initContainers []Container) any {
	p := ImageStrategicPatchCronJob{}
	p.Spec.JobTemplate.Spec.Template.Spec.Containers = containers
	p.Spec.JobTemplate.Spec.Template.Spec.InitContainers = initContainers
	return p
}

func getWorkloadAdapter(obj any) (*workloadAdapter, error) {
	switch t := obj.(type) {
	case *appV1.Deployment:
		return &workloadAdapter{annotations: t.Annotations, podSpec: &t.Spec.Template.Spec, buildPatch: buildDefaultPatch}, nil
	case *appV1.StatefulSet:
		return &workloadAdapter{annotations: t.Annotations, podSpec: &t.Spec.Template.Spec, buildPatch: buildDefaultPatch}, nil
	case *appV1.DaemonSet:
		return &workloadAdapter{annotations: t.Annotations, podSpec: &t.Spec.Template.Spec, buildPatch: buildDefaultPatch}, nil
	case *batchV1.CronJob:
		return &workloadAdapter{annotations: t.Annotations, podSpec: &t.Spec.JobTemplate.Spec.Template.Spec, buildPatch: buildCronJobPatch}, nil
	default:
		return nil, fmt.Errorf("unsupported workload type %T", t)
	}
}

func GetAnnotations(obj any) (map[string]string, error) {
	adapter, err := getWorkloadAdapter(obj)
	if err != nil {
		return make(map[string]string), err
	}
	return adapter.annotations, nil
}

func GetContainers(obj any) ([]coreV1.Container, error) {
	adapter, err := getWorkloadAdapter(obj)
	if err != nil {
		return make([]coreV1.Container, 0), err
	}
	return adapter.podSpec.Containers, nil
}

func GetInitContainers(obj any) ([]coreV1.Container, error) {
	adapter, err := getWorkloadAdapter(obj)
	if err != nil {
		return make([]coreV1.Container, 0), err
	}
	return adapter.podSpec.InitContainers, nil
}

func GetContainerByName(obj any, name string) (coreV1.Container, error) {
	containers, err := GetContainers(obj)

	if err != nil {
		return coreV1.Container{}, err
	}

	for _, container := range containers {
		if container.Name == name {
			return container, nil
		}
	}
	return coreV1.Container{}, fmt.Errorf("container %s not found", name)
}

func GetInitContainerByName(obj any, name string) (coreV1.Container, error) {
	containers, err := GetInitContainers(obj)

	if err != nil {
		return coreV1.Container{}, err
	}

	for _, container := range containers {
		if container.Name == name {
			return container, nil
		}
	}
	return coreV1.Container{}, fmt.Errorf("initContainer %s not found", name)
}

func GetImageStrategicPatchJson(obj any, containers, initContainers []Container) ([]byte, error) {
	adapter, err := getWorkloadAdapter(obj)
	if err != nil {
		return nil, err
	}
	return json.Marshal(adapter.buildPatch(containers, initContainers))
}
