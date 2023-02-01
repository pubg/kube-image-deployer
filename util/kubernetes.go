package util

import (
	"encoding/json"
	"fmt"
	"strings"

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

func GetAnnotations(obj interface{}) (map[string]string, error) {
	switch t := obj.(type) {
	case *appV1.Deployment:
		return t.Annotations, nil
	case *appV1.StatefulSet:
		return t.Annotations, nil
	case *appV1.DaemonSet:
		return t.Annotations, nil
	case *batchV1.CronJob:
		return t.Annotations, nil
	default:
		return make(map[string]string), fmt.Errorf("GetAnnotations unknown type %T", t)
	}
}

func GetContainers(obj interface{}) ([]coreV1.Container, error) {
	switch t := obj.(type) {
	case *appV1.Deployment:
		return t.Spec.Template.Spec.Containers, nil
	case *appV1.StatefulSet:
		return t.Spec.Template.Spec.Containers, nil
	case *appV1.DaemonSet:
		return t.Spec.Template.Spec.Containers, nil
	case *batchV1.CronJob:
		return t.Spec.JobTemplate.Spec.Template.Spec.Containers, nil
	default:
		return make([]coreV1.Container, 0), fmt.Errorf("GetContainers unknown type %T", t)
	}
}

func GetInitContainers(obj interface{}) ([]coreV1.Container, error) {
	switch t := obj.(type) {
	case *appV1.Deployment:
		return t.Spec.Template.Spec.InitContainers, nil
	case *appV1.StatefulSet:
		return t.Spec.Template.Spec.InitContainers, nil
	case *appV1.DaemonSet:
		return t.Spec.Template.Spec.InitContainers, nil
	case *batchV1.CronJob:
		return t.Spec.JobTemplate.Spec.Template.Spec.InitContainers, nil
	default:
		return make([]coreV1.Container, 0), fmt.Errorf("GetInitContainers unknown type %T", t)
	}
}

func GetContainerByName(obj interface{}, name string) (coreV1.Container, error) {
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

func GetInitContainerByName(obj interface{}, name string) (coreV1.Container, error) {
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

func GetNamespaceNameByKey(key string) (namespace string, name string) {
	arr := strings.Split(key, "/")
	if len(arr) == 2 {
		return arr[0], arr[1]
	}
	return
}

func GetImageStrategicPatchJson(obj interface{}, containers, initContainers []Container) ([]byte, error) {
	var imageStrategicPatch interface{}

	switch obj.(type) {
	case *batchV1.CronJob:
		p := ImageStrategicPatchCronJob{}
		p.Spec.JobTemplate.Spec.Template.Spec.Containers = containers
		p.Spec.JobTemplate.Spec.Template.Spec.InitContainers = initContainers
		imageStrategicPatch = p
	default:
		p := ImageStrategicPatch{}
		p.Spec.Template.Spec.Containers = containers
		p.Spec.Template.Spec.InitContainers = initContainers
		imageStrategicPatch = p
	}
	patchJson, err := json.Marshal(imageStrategicPatch)
	return patchJson, err
}
