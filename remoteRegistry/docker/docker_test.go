package docker

import (
	"strings"
	"testing"
)

func TestGetImageStringAsterisk(t *testing.T) {
	r := NewRemoteRegistry()

	if resolved, err := r.GetImageString("busybox", "1.34.*", "linux/amd64"); err != nil {
		t.Fatalf("err: %v", err)
	} else if !strings.HasPrefix(resolved.ImageString, "busybox@sha256:") {
		t.Fatalf("no digest: %+v", resolved)
	} else if !strings.HasPrefix(resolved.Tag, "1.34.") || strings.Contains(resolved.Tag, "*") {
		t.Fatalf("resolved tag must be a concrete matched tag: %+v", resolved)
	} else {
		t.Logf("success: %s (tag=%s)", resolved.ImageString, resolved.Tag)
	}
}
