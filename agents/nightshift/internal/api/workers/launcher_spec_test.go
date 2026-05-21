package workers

import (
	"testing"
	"time"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

// TestBuildLaunchSpec_WorkerExtraEnv pins the chart-driven worker-env
// passthrough: whatever map the operator hands us via WorkerExtraEnv
// reaches LaunchSpec.ExtraEnv verbatim. The API does not interpret
// keys or values — that's the whole point of the seam.
func TestBuildLaunchSpec_WorkerExtraEnv(t *testing.T) {
	t.Run("nil-passthrough", func(t *testing.T) {
		s := &Service{}
		spec := s.buildLaunchSpec(&nsv1.Run{Id: "r1"}, "cred")
		if spec.ExtraEnv != nil {
			t.Errorf("ExtraEnv: got %v, want nil", spec.ExtraEnv)
		}
	})
	t.Run("populated-passthrough", func(t *testing.T) {
		s := &Service{WorkerExtraEnv: map[string]string{
			"NS_OPENBAO_ADDR":      "http://openbao.acme-prod.svc:8200",
			"NS_OPENBAO_AUTH_ROLE": "nightshift-worker",
		}}
		spec := s.buildLaunchSpec(&nsv1.Run{Id: "r1"}, "cred")
		if got, want := spec.ExtraEnv["NS_OPENBAO_ADDR"], "http://openbao.acme-prod.svc:8200"; got != want {
			t.Errorf("NS_OPENBAO_ADDR: got %q, want %q", got, want)
		}
		if got, want := spec.ExtraEnv["NS_OPENBAO_AUTH_ROLE"], "nightshift-worker"; got != want {
			t.Errorf("NS_OPENBAO_AUTH_ROLE: got %q, want %q", got, want)
		}
	})
}

// TestBuildLaunchSpec_TTLInSeconds is a regression test for a bug
// where ttlSecondsAfterFinished() (a time.Duration in nanoseconds)
// was cast directly to int32, overflowing into a negative number and
// causing K8s Job creation to fail with `spec.ttlSecondsAfterFinished:
// must be greater than or equal to 0`. Fix: convert via .Seconds().
func TestBuildLaunchSpec_TTLInSeconds(t *testing.T) {
	cases := []struct {
		name         string
		ttl          time.Duration
		deadline     time.Duration
		wantTTLSec   int32
		wantDeadSecs int64
	}{
		{"defaults", 0, 0, 300, 3600},
		{"explicit-1m-and-10m", time.Minute, 10 * time.Minute, 60, 600},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &Service{TTLAfterFinished: c.ttl, ActiveDeadline: c.deadline}
			spec := s.buildLaunchSpec(&nsv1.Run{Id: "r1"}, "cred")
			if spec.TTLSecondsAfterFinished != c.wantTTLSec {
				t.Errorf("TTL: got %d, want %d", spec.TTLSecondsAfterFinished, c.wantTTLSec)
			}
			if spec.ActiveDeadlineSeconds != c.wantDeadSecs {
				t.Errorf("deadline: got %d, want %d", spec.ActiveDeadlineSeconds, c.wantDeadSecs)
			}
		})
	}
}
