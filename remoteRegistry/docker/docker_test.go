package docker

import (
	"strings"
	"testing"
)

func TestGetImageStringAsterisk(t *testing.T) {
	r := NewRemoteRegistry()

	if s, err := r.GetImageString("busybox", "1.34.*", "linux/amd64"); err != nil {
		t.Fatalf("err: %v", err)
	} else if !strings.HasPrefix(s, "busybox@sha256:") {
		t.Fatalf("no digest: %s", s)
	} else {
		t.Logf("success: %s", s)
	}
}
