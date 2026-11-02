package workflows

import (
	"testing"

	"github.com/khaledhikmat/psap/model"
)

func TestShieldWaitingOnReturnsBusinessManager(t *testing.T) {
	tests := []struct {
		name  string
		state model.ShieldState
		task  string
	}{
		{
			name:  "answers",
			state: model.WaitingForAnswersShieldState,
			task:  "submit the shield exposure ratings",
		},
		{
			name:  "evidence",
			state: model.WaitingForEvidencesShieldState,
			task:  "submit the shield evidence",
		},
		{
			name:  "answers submitted",
			state: model.AnswersSubmittedShieldState,
			task:  "submit the shield evidence",
		},
		{
			name:  "evidence submitted",
			state: model.EvidencesSubmittedShieldState,
			task:  "complete the shield",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &shield{
				req: ShieldRequest{
					BusinessOwner:   "owner-1",
					BusinessManager: "manager-1",
				},
				state: tt.state,
			}

			recipient, task := w.waitingOn()
			if recipient != "manager-1" {
				t.Errorf("waitingOn() recipient = %q, want manager-1", recipient)
			}
			if task != tt.task {
				t.Errorf("waitingOn() task = %q, want %q", task, tt.task)
			}
		})
	}
}
