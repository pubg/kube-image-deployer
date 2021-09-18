package util

import (
	"fmt"
	"strings"

	appV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func GetAnnotations(obj interface{}) map[string]string {
	switch t := obj.(type) {
	case *appV1.Deployment:
		return obj.(*appV1.Deployment).Annotations
	case *appV1.StatefulSet:
		return obj.(*appV1.StatefulSet).Annotations
	case *appV1.DaemonSet:
		return obj.(*appV1.DaemonSet).Annotations
	case *batchV1.CronJob:
		return obj.(*batchV1.CronJob).Annotations
	default:
		klog.Errorf("GetAnnotations unknown type %T", t)
		return make(map[string]string)
	}
}

func GetContainers(obj interface{}) []coreV1.Container {
	switch t := obj.(type) {
	case *appV1.Deployment:
		return obj.(*appV1.Deployment).Spec.Template.Spec.Containers
	case *appV1.StatefulSet:
		return obj.(*appV1.StatefulSet).Spec.Template.Spec.Containers
	case *appV1.DaemonSet:
		return obj.(*appV1.DaemonSet).Spec.Template.Spec.Containers
	case *batchV1.CronJob:
		return obj.(*batchV1.CronJob).Spec.JobTemplate.Spec.Template.Spec.Containers
	default:
		klog.Errorf("GetContainers unknown type %T", t)
		return make([]coreV1.Container, 0)
	}
}

func GetInitContainers(obj interface{}) []coreV1.Container {
	switch t := obj.(type) {
	case *appV1.Deployment:
		return obj.(*appV1.Deployment).Spec.Template.Spec.InitContainers
	case *appV1.StatefulSet:
		return obj.(*appV1.StatefulSet).Spec.Template.Spec.InitContainers
	case *appV1.DaemonSet:
		return obj.(*appV1.DaemonSet).Spec.Template.Spec.InitContainers
	case *batchV1.CronJob:
		return obj.(*batchV1.CronJob).Spec.JobTemplate.Spec.Template.Spec.InitContainers
	default:
		klog.Errorf("GetInitContainers unknown type %T", t)
		return make([]coreV1.Container, 0)
	}
}

func GetContainerByName(obj interface{}, name string) (coreV1.Container, error) {
	containers := GetContainers(obj)
	for _, container := range containers {
		if container.Name == name {
			return container, nil
		}
	}
	return coreV1.Container{}, fmt.Errorf("container %s not found", name)
}

func GetInitContainerByName(obj interface{}, name string) (coreV1.Container, error) {
	containers := GetInitContainers(obj)
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
