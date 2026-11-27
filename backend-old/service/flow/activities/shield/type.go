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
	ShieldID         string
	Questions        int
	EvidenceRequired int // number of questions that require evidence
}

type ShieldAnswersSaveRequest struct {
	ShieldID  string
	Answers   []model.AnswerSubmission
	UpdatedBy string
}

type ShieldAnswersSaveResult struct {
	Answered  int
	Questions int
}

// ShieldAnswersFinalizeRequest carries the last answers. The flow service only lets a shield
// finalize once every question is answered.
type ShieldAnswersFinalizeRequest struct {
	ShieldID  string
	Answers   []model.AnswerSubmission
	UpdatedBy string
}

type ShieldEvidencesSaveRequest struct {
	ShieldID  string
	Evidences []model.EvidenceSubmission
	UpdatedBy string
}

type ShieldEvidencesSaveResult struct {
	Provided int // questions requiring evidence that have some
	Required int // questions requiring evidence
}

type ShieldEvidencesFinalizeRequest struct {
	ShieldID  string
	Evidences []model.EvidenceSubmission
	UpdatedBy string
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
