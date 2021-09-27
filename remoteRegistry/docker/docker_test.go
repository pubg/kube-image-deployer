package docker

import (
	"strings"
	"testing"
)

func TestGetImageStringAsterisk(t *testing.T) {
	r := NewRemoteRegistry()

	if s, err := r.GetImageString("busybox", "1.34.*", "linux/amd64"); err != nil {
		t.Fatalf("TestGetImageStringAsterisk asterisk err: %v", err)
	} else if !strings.Contains(s, "@sha256:") {
		t.Fatalf("TestGetImageStringAsterisk asterisk no digest: %s", s)
	} else {
		t.Logf("TestGetImageStringAsterisk asterisk success: %s", s)
	}
}
