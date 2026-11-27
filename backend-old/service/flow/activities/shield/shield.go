package shieldactivities

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/answers"
	"github.com/khaledhikmat/psap/service/data"
	"go.temporal.io/sdk/temporal"
)

const shieldActor = "shield-workflow"

// Application error types of submissions that cannot be saved. Retrying does not help, so
// activities return them as non-retryable errors.
const (
	ErrInvalidAnswer   = "InvalidAnswer"
	ErrInvalidEvidence = "InvalidEvidence"
)

// CreateShield creates a new Shield record and updates the asset's LastShieldID. Answers and
// evidences are created as the business owner submits them. It returns the shield ID, the
// number of shield questions and how many of them require evidence.
func (a *Activities) CreateShield(ctx context.Context, req ShieldCreationRequest) (ShieldCreationResult, error) {
	shieldID := uuid.NewString()

	asset, err := a.DataSvc.RetrieveAssetByID(ctx, req.AssetID)
	if err != nil {
		return ShieldCreationResult{}, err
	}

	err = a.DataSvc.NewShield(ctx, model.Shield{
		ID:        shieldID,
		AssetID:   req.AssetID,
		State:     model.WaitingForAnswersShieldState,
		CreatedBy: shieldActor,
		UpdatedBy: shieldActor,
	})
	if err != nil && !errors.Is(err, data.ErrAlreadyExists) {
		return ShieldCreationResult{}, err
	}

	questions, err := a.DataSvc.RetrieveQuestionsByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		return ShieldCreationResult{}, err
	}

	asset.LastShieldID = &shieldID
	asset.UpdatedBy = shieldActor
	if err := a.DataSvc.UpdateAsset(ctx, asset); err != nil {
		return ShieldCreationResult{}, err
	}

	return ShieldCreationResult{
		ShieldID:         shieldID,
		Questions:        len(questions),
		EvidenceRequired: countEvidenceRequired(questions),
	}, nil
}

// SaveShieldAnswers records the selected predefined answers of the submitted questions. Partial
// submissions are allowed: questions not in the request are left unchanged. This can be
// called multiple times.
func (a *Activities) SaveShieldAnswers(ctx context.Context, req ShieldAnswersSaveRequest) (ShieldAnswersSaveResult, error) {
	questions, err := a.DataSvc.RetrieveQuestionsByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		return ShieldAnswersSaveResult{}, err
	}
	predefined, err := a.DataSvc.RetrievePredefinedAnswersByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		return ShieldAnswersSaveResult{}, err
	}
	if err := answers.Validate(questions, predefined, req.Answers); err != nil {
		return ShieldAnswersSaveResult{}, temporal.NewNonRetryableApplicationError(err.Error(), ErrInvalidAnswer, err)
	}

	if err := a.DataSvc.SaveShieldAnswers(ctx, req.ShieldID, req.Answers, actorOr(req.UpdatedBy)); err != nil {
		return ShieldAnswersSaveResult{}, err
	}
	saved, err := a.DataSvc.RetrieveUserAnswersByShieldID(ctx, req.ShieldID)
	if err != nil {
		return ShieldAnswersSaveResult{}, err
	}
	return ShieldAnswersSaveResult{Answered: answers.Answered(saved), Questions: len(questions)}, nil
}

// FinalizeAnswers records any last answers and locks them.
func (a *Activities) FinalizeAnswers(ctx context.Context, req ShieldAnswersFinalizeRequest) (ShieldAnswersSaveResult, error) {
	return a.SaveShieldAnswers(ctx, ShieldAnswersSaveRequest(req))
}

// SaveEvidences records the given evidences, each for an already answered question. Partial
// submissions are allowed: questions not in the request are left unchanged. This can be
// called multiple times.
func (a *Activities) SaveEvidences(ctx context.Context, req ShieldEvidencesSaveRequest) (ShieldEvidencesSaveResult, error) {
	questions, err := a.DataSvc.RetrieveQuestionsByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		return ShieldEvidencesSaveResult{}, err
	}
	known := make(map[string]bool, len(questions))
	for _, q := range questions {
		known[q.ID] = true
	}
	answered, err := a.DataSvc.RetrieveUserAnswersByShieldID(ctx, req.ShieldID)
	if err != nil {
		return ShieldEvidencesSaveResult{}, err
	}
	answerByQuestion := make(map[string]model.UserAnswerDetail, len(answered))
	for _, ua := range answered {
		answerByQuestion[ua.QuestionID] = ua
	}
	evidences, err := a.DataSvc.RetrieveEvidencesByShieldID(ctx, req.ShieldID)
	if err != nil {
		return ShieldEvidencesSaveResult{}, err
	}
	byAnswer := make(map[string]model.Evidence, len(evidences))
	for _, e := range evidences {
		byAnswer[e.UserAnswerID] = e
	}

	actor := actorOr(req.UpdatedBy)
	created := []model.Evidence{}
	changed := []model.Evidence{}
	for _, sub := range req.Evidences {
		if !known[sub.QuestionID] {
			return ShieldEvidencesSaveResult{}, temporal.NewNonRetryableApplicationError(
				fmt.Sprintf("%q is not a shield question", sub.QuestionID), ErrInvalidEvidence, nil)
		}
		ua, ok := answerByQuestion[sub.QuestionID]
		if !ok || len(ua.Selections) == 0 {
			return ShieldEvidencesSaveResult{}, temporal.NewNonRetryableApplicationError(
				fmt.Sprintf("question %q has no answer to attach evidence to", sub.QuestionID), ErrInvalidEvidence, nil)
		}

		existing, found := byAnswer[ua.ID]
		if !found {
			shieldID := req.ShieldID
			created = append(created, model.Evidence{
				ID:           ua.ID + "-evidence",
				QuestionID:   sub.QuestionID,
				UserAnswerID: ua.ID,
				ShieldID:     &shieldID,
				Description:  sub.Description,
				URL:          sub.URL,
				CreatedBy:    actor,
				UpdatedBy:    actor,
			})
			continue
		}
		if existing.Description == sub.Description && existing.URL == sub.URL {
			continue
		}
		existing.Description = sub.Description
		existing.URL = sub.URL
		existing.UpdatedBy = actor
		changed = append(changed, existing)
	}

	if len(created) > 0 {
		if err := a.DataSvc.NewEvidences(ctx, created); err != nil {
			return ShieldEvidencesSaveResult{}, err
		}
	}
	if len(changed) > 0 {
		if err := a.DataSvc.UpdateEvidences(ctx, changed); err != nil {
			return ShieldEvidencesSaveResult{}, err
		}
	}

	evidences, err = a.DataSvc.RetrieveEvidencesByShieldID(ctx, req.ShieldID)
	if err != nil {
		return ShieldEvidencesSaveResult{}, err
	}
	return evidenceStatus(questions, evidences), nil
}

// FinalizeEvidences records any last evidences and locks them.
func (a *Activities) FinalizeEvidences(ctx context.Context, req ShieldEvidencesFinalizeRequest) (ShieldEvidencesSaveResult, error) {
	return a.SaveEvidences(ctx, ShieldEvidencesSaveRequest(req))
}

// CalculateScore sums up the points of the shield's selected answers to compute the shield score.
func (a *Activities) CalculateScore(ctx context.Context, req ShieldScoreCalculationRequest) (ShieldScoreCalculationResult, error) {
	saved, err := a.DataSvc.RetrieveUserAnswersByShieldID(ctx, req.ShieldID)
	if err != nil {
		return ShieldScoreCalculationResult{}, err
	}
	predefined, err := a.DataSvc.RetrievePredefinedAnswersByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		return ShieldScoreCalculationResult{}, err
	}

	return ShieldScoreCalculationResult{
		Score: answers.Points(saved, predefined),
	}, nil
}

// PersistShield stores the shield's current score, state and nudge counts. Once evidences are
// finalized, it also updates the asset to 'shielded' state.
func (a *Activities) PersistShield(ctx context.Context, req ShieldPersistRequest) error {
	shield, err := a.DataSvc.RetrieveShieldByID(ctx, req.ShieldID)
	if errors.Is(err, data.ErrNotFound) {
		// E.g. the shield was cancelled before Create stored it; retrying will not help.
		return temporal.NewNonRetryableApplicationError(err.Error(), "ShieldNotFound", err)
	}
	if err != nil {
		return err
	}
	shield.Score = req.Score
	shield.State = req.State
	shield.Reminders = req.Reminders
	shield.Warnings = req.Warnings
	shield.Escalations = req.Escalations
	shield.UpdatedBy = shieldActor
	if err := a.DataSvc.UpdateShield(ctx, shield); err != nil {
		return err
	}

	if req.State != model.CompletedShieldState {
		return nil
	}

	asset, err := a.DataSvc.RetrieveAssetByID(ctx, shield.AssetID)
	if err != nil {
		return err
	}
	asset.State = model.ShieldedAssetState
	asset.UpdatedBy = shieldActor
	return a.DataSvc.UpdateAsset(ctx, asset)
}

func actorOr(updatedBy string) string {
	if updatedBy == "" {
		return shieldActor
	}
	return updatedBy
}

func countEvidenceRequired(questions []model.Question) int {
	n := 0
	for _, q := range questions {
		if q.EvidenceRequired {
			n++
		}
	}
	return n
}

// evidenceStatus counts the questions that require evidence, and how many of them have some.
func evidenceStatus(questions []model.Question, evidences []model.Evidence) ShieldEvidencesSaveResult {
	provided := make(map[string]bool, len(evidences))
	for _, e := range evidences {
		if e.Description != "" || e.URL != "" {
			provided[e.QuestionID] = true
		}
	}
	res := ShieldEvidencesSaveResult{}
	for _, q := range questions {
		if !q.EvidenceRequired {
			continue
		}
		res.Required++
		if provided[q.ID] {
			res.Provided++
		}
	}
	return res
}
