package workers

import (
	"time"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/runtime"
)

// buildLaunchSpec maps a persisted Run + minted worker credential
// into a runtime.LaunchSpec.
func (s *Service) buildLaunchSpec(run *nsv1.Run, credential string) runtime.LaunchSpec {
	ttl := int32(s.ttlSecondsAfterFinished().Seconds())
	deadline := int64(s.defaultActiveDeadline().Seconds())
	return runtime.LaunchSpec{
		RunID:                    run.GetId(),
		UserID:                   run.GetUserId(),
		SessionID:                run.GetSessionId(),
		Prompt:                   run.GetPrompt(),
		Image:                    s.WorkerImage,
		CallbackURL:              s.CallbackURL,
		WorkerCredential:         credential,
		TTLSecondsAfterFinished:  ttl,
		ActiveDeadlineSeconds:    deadline,
		SessionState:             s.SessionState,
		MountServiceAccountToken: s.MountWorkerServiceAccountToken,
		ExtraEnv:                 s.WorkerExtraEnv,
	}
}

func (s *Service) ttlSecondsAfterFinished() time.Duration {
	if s.TTLAfterFinished > 0 {
		return s.TTLAfterFinished
	}
	return 300 * time.Second
}

func (s *Service) defaultActiveDeadline() time.Duration {
	if s.ActiveDeadline > 0 {
		return s.ActiveDeadline
	}
	return time.Hour
}
