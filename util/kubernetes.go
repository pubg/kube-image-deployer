package util

import (
	"fmt"
	"strings"

	appV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
)

func GetAnnotations(obj interface{}) map[string]string {
	if o, ok := obj.(*appV1.Deployment); ok {
		return o.Annotations
	} else if o, ok := obj.(*appV1.StatefulSet); ok {
		return o.Annotations
	} else if o, ok := obj.(*appV1.DaemonSet); ok {
		return o.Annotations
	} else if o, ok := obj.(*batchV1.CronJob); ok {
		return o.Annotations
	}
	return make(map[string]string)
}

func GetContainers(obj interface{}) []coreV1.Container {
	if deployment, ok := obj.(*appV1.Deployment); ok {
		return deployment.Spec.Template.Spec.Containers
	} else if statefulSet, ok := obj.(*appV1.StatefulSet); ok {
		return statefulSet.Spec.Template.Spec.Containers
	} else if daemonSet, ok := obj.(*appV1.DaemonSet); ok {
		return daemonSet.Spec.Template.Spec.Containers
	} else if cronjob, ok := obj.(*batchV1.CronJob); ok {
		return cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers
	}
	return make([]coreV1.Container, 0)
}

func GetInitContainers(obj interface{}) []coreV1.Container {
	if deployment, ok := obj.(*appV1.Deployment); ok {
		return deployment.Spec.Template.Spec.InitContainers
	} else if statefulSet, ok := obj.(*appV1.StatefulSet); ok {
		return statefulSet.Spec.Template.Spec.InitContainers
	} else if daemonSet, ok := obj.(*appV1.DaemonSet); ok {
		return daemonSet.Spec.Template.Spec.InitContainers
	} else if cronjob, ok := obj.(*batchV1.CronJob); ok {
		return cronjob.Spec.JobTemplate.Spec.Template.Spec.InitContainers
	}
	return make([]coreV1.Container, 0)
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
