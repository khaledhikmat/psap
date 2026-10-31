package shieldactivities

import (
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/data"
)

type Activities struct {
	DataSvc data.IService
}

type ShieldCreationRequest struct {
	AssetID string
}

type ShieldCreationResult struct {
	ShieldID   string
	Questions  int
	Categories int
}

type ShieldAnswersSaveRequest struct {
	ShieldID string
	Answers  map[string]string // questionID -> reply
}

type ShieldAnswersFinalizeRequest struct {
	ShieldID string
	Answers  map[string]string // questionID -> reply (remaining unanswered become NO)
}

type ShieldEvidencesSaveRequest struct {
	ShieldID  string
	Evidences map[string]ShieldEvidenceData // categoryID -> evidence
}

type ShieldEvidenceData struct {
	Description string
	URL         string
}

type ShieldEvidencesFinalizeRequest struct {
	ShieldID  string
	Evidences map[string]ShieldEvidenceData // categoryID -> evidence
}

type ShieldScoreCalculationRequest struct {
	ShieldID string
}

type ShieldScoreCalculationResult struct {
	Score int
}

type ShieldPersistRequest struct {
	ShieldID    string
	Score       int
	State       model.ShieldState
	Reminders   int
	Warnings    int
	Escalations int
}
