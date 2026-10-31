package shieldactivities

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/data"
	"go.temporal.io/sdk/temporal"
)

const shieldActor = "shield-workflow"

// Create creates a new Shield record, creates unanswered Answers for all shield questions,
// and creates empty Evidence records for all shield categories. It updates the asset's
// LastShieldID and returns the shield ID and counts.
func (a *Activities) Create(ctx context.Context, req ShieldCreationRequest) (ShieldCreationResult, error) {
	shieldID := uuid.NewString()

	asset, err := a.DataSvc.RetrieveAssetByID(ctx, req.AssetID)
	if err != nil {
		return ShieldCreationResult{}, err
	}

	// Create Shield record
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

	// Create unanswered Answers for all shield questions
	questions, err := a.DataSvc.RetrieveQuestionsByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		return ShieldCreationResult{}, err
	}
	answers := make([]model.Answer, len(questions))
	for i, q := range questions {
		answers[i] = model.Answer{
			ID:         shieldID + "-" + q.ID,
			QuestionID: q.ID,
			ShieldID:   &shieldID,
			Reply:      model.UnansweredReply,
			CreatedBy:  shieldActor,
			UpdatedBy:  shieldActor,
		}
	}
	err = a.DataSvc.NewAnswers(ctx, answers)
	if err != nil && !errors.Is(err, data.ErrAlreadyExists) {
		return ShieldCreationResult{}, err
	}

	// Get all shield question categories to create empty evidences
	categories, err := a.DataSvc.RetrieveQuestionCategoriesByType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		return ShieldCreationResult{}, err
	}

	// Create empty Evidence records for each category
	evidences := make([]model.Evidence, len(categories))
	for i, cat := range categories {
		evidences[i] = model.Evidence{
			ID:                 shieldID + "-" + cat.ID,
			QuestionCategoryID: cat.ID,
			ShieldID:           shieldID,
			Description:        "",
			URL:                "",
			CreatedBy:          shieldActor,
			UpdatedBy:          shieldActor,
		}
	}
	err = a.DataSvc.NewEvidences(ctx, evidences)
	if err != nil && !errors.Is(err, data.ErrAlreadyExists) {
		return ShieldCreationResult{}, err
	}

	// Update asset's LastShieldID
	asset.LastShieldID = &shieldID
	asset.UpdatedBy = shieldActor
	if err := a.DataSvc.UpdateAsset(ctx, asset); err != nil {
		return ShieldCreationResult{}, err
	}

	return ShieldCreationResult{
		ShieldID:   shieldID,
		Questions:  len(questions),
		Categories: len(categories),
	}, nil
}

// SaveAnswers records the given replies on the shield's answers. Partial submissions are
// allowed: questions not in the request are left unchanged. This can be called multiple times.
func (a *Activities) SaveAnswers(ctx context.Context, req ShieldAnswersSaveRequest) error {
	answers, err := a.DataSvc.RetrieveAnswersByShieldID(ctx, req.ShieldID)
	if err != nil {
		return err
	}

	changed := []model.Answer{}
	for i := range answers {
		reply, ok := req.Answers[answers[i].QuestionID]
		if !ok {
			continue
		}
		normalized, err := NormalizeReply(reply)
		if err != nil {
			return temporal.NewNonRetryableApplicationError(err.Error(), "InvalidReply", err)
		}
		if answers[i].Reply == normalized {
			continue
		}
		answers[i].Reply = normalized
		answers[i].UpdatedBy = shieldActor
		changed = append(changed, answers[i])
	}

	if len(changed) == 0 {
		return nil
	}
	return a.DataSvc.UpdateAnswers(ctx, changed)
}

// FinalizeAnswers records any last replies, then marks every unanswered question as NO and
// locks the answers.
func (a *Activities) FinalizeAnswers(ctx context.Context, req ShieldAnswersFinalizeRequest) error {
	if err := a.SaveAnswers(ctx, ShieldAnswersSaveRequest{
		ShieldID: req.ShieldID,
		Answers:  req.Answers,
	}); err != nil {
		return err
	}

	answers, err := a.DataSvc.RetrieveAnswersByShieldID(ctx, req.ShieldID)
	if err != nil {
		return err
	}

	changed := []model.Answer{}
	for i := range answers {
		if answers[i].Reply == model.UnansweredReply {
			answers[i].Reply = model.NoReply
			answers[i].UpdatedBy = shieldActor
			changed = append(changed, answers[i])
		}
	}

	if len(changed) > 0 {
		if err := a.DataSvc.UpdateAnswers(ctx, changed); err != nil {
			return err
		}
	}

	return nil
}

// SaveEvidences records the given evidences for the shield's categories. Partial submissions
// are allowed: categories not in the request are left unchanged. This can be called multiple times.
func (a *Activities) SaveEvidences(ctx context.Context, req ShieldEvidencesSaveRequest) error {
	evidences, err := a.DataSvc.RetrieveEvidencesByShieldID(ctx, req.ShieldID)
	if err != nil {
		return err
	}

	changed := []model.Evidence{}
	for i := range evidences {
		evidData, ok := req.Evidences[evidences[i].QuestionCategoryID]
		if !ok {
			continue
		}
		// Only update if changed
		if evidences[i].Description == evidData.Description && evidences[i].URL == evidData.URL {
			continue
		}
		evidences[i].Description = evidData.Description
		evidences[i].URL = evidData.URL
		evidences[i].UpdatedBy = shieldActor
		changed = append(changed, evidences[i])
	}

	if len(changed) == 0 {
		return nil
	}
	return a.DataSvc.UpdateEvidences(ctx, changed)
}

// FinalizeEvidences records any last evidences and locks them.
func (a *Activities) FinalizeEvidences(ctx context.Context, req ShieldEvidencesFinalizeRequest) error {
	return a.SaveEvidences(ctx, ShieldEvidencesSaveRequest{
		ShieldID:  req.ShieldID,
		Evidences: req.Evidences,
	})
}

// CalculateScore sums up the weights of all YES answers to compute the shield score.
func (a *Activities) CalculateScore(ctx context.Context, req ShieldScoreCalculationRequest) (ShieldScoreCalculationResult, error) {
	answers, err := a.DataSvc.RetrieveAnswersByShieldID(ctx, req.ShieldID)
	if err != nil {
		return ShieldScoreCalculationResult{}, err
	}

	score := 0
	for _, answer := range answers {
		if answer.Reply != model.YesReply {
			continue
		}
		question, err := a.DataSvc.RetrieveQuestionByID(ctx, answer.QuestionID)
		if err != nil {
			return ShieldScoreCalculationResult{}, err
		}
		score += question.Weight
	}

	return ShieldScoreCalculationResult{
		Score: score,
	}, nil
}

// Persist stores the shield's current score, state and nudge counts. Once evidences are
// finalized, it also updates the asset to 'shielded' state.
func (a *Activities) Persist(ctx context.Context, req ShieldPersistRequest) error {
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

// NormalizeReply accepts yes/no in any case (and "" for unanswered) and returns the stored form.
func NormalizeReply(reply string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(reply)) {
	case "":
		return model.UnansweredReply, nil
	case "YES", "Y":
		return model.YesReply, nil
	case "NO", "N":
		return model.NoReply, nil
	default:
		return "", fmt.Errorf("invalid reply %q; must be YES or NO", reply)
	}
}
