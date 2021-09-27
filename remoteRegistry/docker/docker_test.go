package docker

import (
	"strings"
	"testing"
)

func TestDocker(t *testing.T) {
	r := NewRemoteRegistry()

	if s, err := r.GetImageString("busybox", "1.34.*", "linux/amd64"); err != nil {
		t.Fatalf("GetImageString asterisk err: %v", err)
	} else if !strings.Contains(s, "@sha256:") {
		t.Fatalf("GetImageString asterisk no digest: %s", s)
	} else {
		t.Logf("GetImageString asterisk success: %s", s)
	}

}
