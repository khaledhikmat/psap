package launch

import "github.com/khaledhikmat/psap/model"

type LaunchCreationRequest struct {
	AssetID string
	// LaunchID is set by minions. Temporal workflows leave it empty and use their run ID.
	LaunchID string
}

type LaunchCreationResult struct {
	LaunchID  string
	Questions int // number of launch questions to answer
}

type LaunchAnswersSaveRequest struct {
	LaunchID  string
	Answers   []model.AnswerSubmission
	UpdatedBy string
}

type LaunchAnswersSaveResult struct {
	Answered  int
	Questions int
}

type LaunchTierCalculationRequest struct {
	LaunchID string
}

type LaunchTierCalculationResult struct {
	Points int
	TierID string
}

type LaunchPersistRequest struct {
	LaunchID    string
	TierID      string
	State       model.LaunchState
	Reminders   int
	Warnings    int
	Escalations int
}
