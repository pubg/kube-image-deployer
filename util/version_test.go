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
