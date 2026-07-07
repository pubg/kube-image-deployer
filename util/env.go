package util

import (
	"os"
	"strconv"

	"k8s.io/klog/v2"
)

// GetEnvBool name 환경변수를 bool로 파싱한다.
// 설정되지 않았으면 fallback을, 파싱할 수 없는 값이면 경고 로그 후 fallback을 반환한다.
// (과거에는 비어있지 않은 모든 값을 true로 처리했으므로, yes/on 등을 쓰던 환경은 동작이 바뀐다)
func GetEnvBool(name string, fallback bool) bool {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	klog.Warningf("invalid bool value %q for env %s, using fallback %v", v, name, fallback)
	return fallback
}

// GetEnvUint name 환경변수를 uint로 파싱한다.
// 설정되지 않았으면 fallback을, 파싱할 수 없는 값이면 경고 로그 후 fallback을 반환한다.
func GetEnvUint(name string, fallback uint) uint {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	if u, err := strconv.ParseUint(v, 10, 32); err == nil {
		return uint(u)
	}
	klog.Warningf("invalid uint value %q for env %s, using fallback %d", v, name, fallback)
	return fallback
}

// GetEnvString name 환경변수를 반환한다. 설정되지 않았으면 fallback을 반환한다.
func GetEnvString(name string, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
