package util

import (
	"testing"
)

var versions []string = []string{"v1.0.0", "v1.0.1", "v1.0.2", "v1.0.9", "v1.0.10", "v1.0.11", "v2.0.99", "v2.0.9", "v3.1.999", "v3.1.998", "v4.0.999", "v4.1.0", "v5.0.0", "v6.0.0", "v6.0.1.0"}

func TestGetHighestVersionWithFilter(t *testing.T) {
	if highestVersion, _ := GetHighestVersionWithFilter(versions, "v1.0.*"); highestVersion != "v1.0.11" {
		t.Errorf("Expected: v1.0.11, Got: %s", highestVersion)
	}

	if highestVersion, _ := GetHighestVersionWithFilter(versions, "v2.0.*"); highestVersion != "v2.0.99" {
		t.Errorf("Expected: v2.0.99, Got: %s", highestVersion)
	}

	if highestVersion, _ := GetHighestVersionWithFilter(versions, "v3.1.*"); highestVersion != "v3.1.999" {
		t.Errorf("Expected: v3.1.999, Got: %s", highestVersion)
	}
}

func TestGetHighestVersionWithFilterSingleVersion(t *testing.T) {
	if highestVersion, _ := GetHighestVersionWithFilter(versions, "v5.*.*"); highestVersion != "v5.0.0" {
		t.Errorf("Expected: v5.0.0, Got: %s", highestVersion)
	}
}

func TestGetHighestVersionWithFilterMultipleAsterisk(t *testing.T) {
	if highestVersion, _ := GetHighestVersionWithFilter(versions, "v4.*.*"); highestVersion != "v4.1.0" {
		t.Errorf("Expected: v4.1.0, Got: %s", highestVersion)
	}
}

func TestGetHighestVersionWithFilterAsteriskNotMatch(t *testing.T) {
	if highestVersion, _ := GetHighestVersionWithFilter(versions, "v6.*.*"); highestVersion != "v6.0.0" {
		t.Errorf("Expected: v6.0.0, Got: %s", highestVersion)
	}
}

// 레지스트리가 반환하는 태그 순서는 보장되지 않는다.
// 어떤 순서로 주어져도 사전식 비교로 가장 높은 버전을 선택해야 한다.
func TestGetHighestVersionWithFilterOrderIndependent(t *testing.T) {
	cases := []struct {
		versions []string
		filter   string
		expected string
	}{
		{[]string{"2.0", "1.9"}, "*.*", "2.0"}, // 높은 major가 먼저 와도 유지되어야 함
		{[]string{"1.9", "2.0"}, "*.*", "2.0"}, // 역순도 동일
		{[]string{"1.10.0", "1.9.9"}, "1.*.*", "1.10.0"},
		{[]string{"1.9.9", "1.10.0"}, "1.*.*", "1.10.0"},
		{[]string{"3.0.0", "2.99.99", "1.0.0"}, "*.*.*", "3.0.0"},
		{[]string{"1.2.10", "1.2.9", "1.2.2"}, "1.2.*", "1.2.10"},
	}

	for _, c := range cases {
		if got, err := GetHighestVersionWithFilter(c.versions, c.filter); err != nil {
			t.Errorf("versions=%v filter=%s unexpected err: %v", c.versions, c.filter, err)
		} else if got != c.expected {
			t.Errorf("versions=%v filter=%s Expected: %s, Got: %s", c.versions, c.filter, c.expected, got)
		}
	}
}

// 자릿수 하나가 int64를 넘는 비정상 태그는 전체 실패가 아니라 해당 태그만 건너뛰어야 한다.
func TestGetHighestVersionWithFilterSkipInvalidNumber(t *testing.T) {
	if got, err := GetHighestVersionWithFilter([]string{"99999999999999999999", "1.0"}, "*"); err != nil || got != "1" {
		// filter "*"는 "1.0"과 매치되지 않으므로 이 케이스는 매치 자체가 없어야 정상
		if err == nil {
			t.Errorf("Expected ErrNotFound or skip, Got: %s", got)
		}
	}
	if got, err := GetHighestVersionWithFilter([]string{"99999999999999999999", "3"}, "*"); err != nil || got != "3" {
		t.Errorf("overflow tag should be skipped. Expected: 3, Got: %s, err: %v", got, err)
	}
}
