package util

import "testing"

// OFF_DEPLOYMENTS=false 처럼 명시적으로 false를 준 경우 false로 파싱되어야 한다.
// (기존 main.go는 값이 무엇이든 비어있지만 않으면 true로 처리했다)
func TestGetEnvBool(t *testing.T) {

	t.Setenv("TEST_ENV_BOOL", "false")
	if GetEnvBool("TEST_ENV_BOOL", true) != false {
		t.Error("explicit false must be parsed as false")
	}

	t.Setenv("TEST_ENV_BOOL", "true")
	if GetEnvBool("TEST_ENV_BOOL", false) != true {
		t.Error("explicit true must be parsed as true")
	}

	t.Setenv("TEST_ENV_BOOL", "1")
	if GetEnvBool("TEST_ENV_BOOL", false) != true {
		t.Error("1 must be parsed as true")
	}

	t.Setenv("TEST_ENV_BOOL", "not-a-bool")
	if GetEnvBool("TEST_ENV_BOOL", false) != false {
		t.Error("unparseable value must fall back")
	}

	if GetEnvBool("TEST_ENV_BOOL_UNSET", true) != true {
		t.Error("unset env must fall back")
	}

}

func TestGetEnvUint(t *testing.T) {

	t.Setenv("TEST_ENV_UINT", "123")
	if GetEnvUint("TEST_ENV_UINT", 1) != 123 {
		t.Error("valid uint must be parsed")
	}

	t.Setenv("TEST_ENV_UINT", "-1")
	if GetEnvUint("TEST_ENV_UINT", 7) != 7 {
		t.Error("negative value must fall back")
	}

	t.Setenv("TEST_ENV_UINT", "abc")
	if GetEnvUint("TEST_ENV_UINT", 7) != 7 {
		t.Error("unparseable value must fall back")
	}

	if GetEnvUint("TEST_ENV_UINT_UNSET", 42) != 42 {
		t.Error("unset env must fall back")
	}

}

func TestGetEnvString(t *testing.T) {

	t.Setenv("TEST_ENV_STR", "hello")
	if GetEnvString("TEST_ENV_STR", "fallback") != "hello" {
		t.Error("set env must be returned")
	}

	if GetEnvString("TEST_ENV_STR_UNSET", "fallback") != "fallback" {
		t.Error("unset env must fall back")
	}

}
