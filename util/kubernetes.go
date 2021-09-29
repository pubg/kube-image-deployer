package util

import (
	"fmt"
	"strings"

	appV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
)

func GetAnnotations(obj interface{}) (map[string]string, error) {
	switch t := obj.(type) {
	case *appV1.Deployment:
		return obj.(*appV1.Deployment).Annotations, nil
	case *appV1.StatefulSet:
		return obj.(*appV1.StatefulSet).Annotations, nil
	case *appV1.DaemonSet:
		return obj.(*appV1.DaemonSet).Annotations, nil
	case *batchV1.CronJob:
		return obj.(*batchV1.CronJob).Annotations, nil
	default:
		return make(map[string]string), fmt.Errorf("GetAnnotations unknown type %T", t)
	}
}

func GetContainers(obj interface{}) ([]coreV1.Container, error) {
	switch t := obj.(type) {
	case *appV1.Deployment:
		return obj.(*appV1.Deployment).Spec.Template.Spec.Containers, nil
	case *appV1.StatefulSet:
		return obj.(*appV1.StatefulSet).Spec.Template.Spec.Containers, nil
	case *appV1.DaemonSet:
		return obj.(*appV1.DaemonSet).Spec.Template.Spec.Containers, nil
	case *batchV1.CronJob:
		return obj.(*batchV1.CronJob).Spec.JobTemplate.Spec.Template.Spec.Containers, nil
	default:
		return make([]coreV1.Container, 0), fmt.Errorf("GetContainers unknown type %T", t)
	}
}

func GetInitContainers(obj interface{}) ([]coreV1.Container, error) {
	switch t := obj.(type) {
	case *appV1.Deployment:
		return obj.(*appV1.Deployment).Spec.Template.Spec.InitContainers, nil
	case *appV1.StatefulSet:
		return obj.(*appV1.StatefulSet).Spec.Template.Spec.InitContainers, nil
	case *appV1.DaemonSet:
		return obj.(*appV1.DaemonSet).Spec.Template.Spec.InitContainers, nil
	case *batchV1.CronJob:
		return obj.(*batchV1.CronJob).Spec.JobTemplate.Spec.Template.Spec.InitContainers, nil
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
