package data

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/answers"
)

func (s *sqliteService) InitiateLaunch(ctx context.Context, launch model.Launch) error {
	if launch.ID == "" || launch.AssetID == "" {
		return errors.New("launch ID and asset ID are required")
	}
	launch.State = model.WaitingForAnswersLaunchState
	if launch.CreatedBy == "" {
		launch.CreatedBy = "data-service"
	}
	if launch.UpdatedBy == "" {
		launch.UpdatedBy = launch.CreatedBy
	}
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		asset, err := getRow[model.Asset](ctx, tx, assetsTable, "ASSET_ID = ?", launch.AssetID)
		if err != nil {
			return err
		}
		if asset.BusinessOwnerID == nil || asset.BusinessManagerID == nil {
			return fmt.Errorf("%w: asset needs a business owner and manager before it can be launched", ErrInvalidState)
		}
		previous, err := listRows[model.Launch](ctx, tx, launchesTable, "ASSET_ID = ?", launch.AssetID)
		if err != nil {
			return err
		}
		for _, old := range previous {
			if launchIsTerminal(old.State) {
				continue
			}
			old.State = model.CancelledLaunchState
			old.UpdatedBy = launch.UpdatedBy
			if err := updateRow(ctx, tx, launchesTable, old); err != nil {
				return err
			}
		}
		if err := insertRow(ctx, tx, launchesTable, launch); err != nil {
			return err
		}
		asset.LastLaunchID = &launch.ID
		asset.UpdatedBy = launch.UpdatedBy
		return updateRow(ctx, tx, assetsTable, asset)
	})
}

func (s *sqliteService) ValidateLaunchTier(ctx context.Context, launchID, tierID, actor string) error {
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		launch, err := getRow[model.Launch](ctx, tx, launchesTable, "ID = ?", launchID)
		if err != nil {
			return err
		}
		if launch.State != model.WaitingForTierApprovalLaunchState {
			return fmt.Errorf("%w: launch is %s, want %s", ErrInvalidState, launch.State, model.WaitingForTierApprovalLaunchState)
		}
		if tierID != "" {
			if _, err := getRow[model.Tier](ctx, tx, tiersTable, "ID = ?", tierID); err != nil {
				return err
			}
			launch.TierID = &tierID
		}
		if launch.TierID == nil || *launch.TierID == "" {
			return fmt.Errorf("%w: launch has no calculated tier", ErrInvalidState)
		}
		launch.State = model.TierValidatedLaunchState
		launch.UpdatedBy = actorOrDefault(actor, "data-service")
		if err := updateRow(ctx, tx, launchesTable, launch); err != nil {
			return err
		}
		asset, err := getRow[model.Asset](ctx, tx, assetsTable, "ASSET_ID = ?", launch.AssetID)
		if err != nil {
			return err
		}
		asset.TierID = launch.TierID
		asset.State = model.PendingShieldAssetState
		asset.UpdatedBy = launch.UpdatedBy
		return updateRow(ctx, tx, assetsTable, asset)
	})
}

func (s *sqliteService) InitiateShield(ctx context.Context, shield model.Shield) error {
	if shield.ID == "" || shield.AssetID == "" {
		return errors.New("shield ID and asset ID are required")
	}
	shield.State = model.WaitingForAnswersShieldState
	if shield.CreatedBy == "" {
		shield.CreatedBy = "data-service"
	}
	if shield.UpdatedBy == "" {
		shield.UpdatedBy = shield.CreatedBy
	}
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		asset, err := getRow[model.Asset](ctx, tx, assetsTable, "ASSET_ID = ?", shield.AssetID)
		if err != nil {
			return err
		}
		if asset.BusinessOwnerID == nil || asset.BusinessManagerID == nil {
			return fmt.Errorf("%w: asset needs a business owner and manager before it can be shielded", ErrInvalidState)
		}
		previous, err := listRows[model.Shield](ctx, tx, shieldsTable, "ASSET_ID = ?", shield.AssetID)
		if err != nil {
			return err
		}
		for _, old := range previous {
			if shieldIsTerminal(old.State) {
				continue
			}
			old.State = model.CancelledShieldState
			old.UpdatedBy = shield.UpdatedBy
			if err := updateRow(ctx, tx, shieldsTable, old); err != nil {
				return err
			}
		}
		if err := insertRow(ctx, tx, shieldsTable, shield); err != nil {
			return err
		}
		asset.LastShieldID = &shield.ID
		asset.UpdatedBy = shield.UpdatedBy
		return updateRow(ctx, tx, assetsTable, asset)
	})
}

func (s *sqliteService) CompleteShield(ctx context.Context, shieldID string, score int, actor string) error {
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		shield, err := getRow[model.Shield](ctx, tx, shieldsTable, "ID = ?", shieldID)
		if err != nil {
			return err
		}
		if shield.State != model.WaitingForEvidencesShieldState {
			return fmt.Errorf("%w: shield is %s, want %s", ErrInvalidState, shield.State, model.WaitingForEvidencesShieldState)
		}
		shield.Score = score
		shield.State = model.CompletedShieldState
		shield.UpdatedBy = actorOrDefault(actor, "data-service")
		if err := updateRow(ctx, tx, shieldsTable, shield); err != nil {
			return err
		}
		asset, err := getRow[model.Asset](ctx, tx, assetsTable, "ASSET_ID = ?", shield.AssetID)
		if err != nil {
			return err
		}
		asset.State = model.ShieldedAssetState
		asset.UpdatedBy = shield.UpdatedBy
		return updateRow(ctx, tx, assetsTable, asset)
	})
}

func (s *sqliteService) SaveShieldEvidences(ctx context.Context, shieldID string, submissions []model.EvidenceSubmission, actor string) error {
	actor = actorOrDefault(actor, "data-service")
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		shield, err := getRow[model.Shield](ctx, tx, shieldsTable, "ID = ?", shieldID)
		if err != nil {
			return err
		}
		if shield.State != model.WaitingForEvidencesShieldState {
			return fmt.Errorf("%w: shield is %s, want %s", ErrInvalidState, shield.State, model.WaitingForEvidencesShieldState)
		}
		questions, err := listRows[model.Question](ctx, tx, questionsTable,
			"CATEGORY_ID IN (SELECT ID FROM "+questionCategoriesTable.name+" WHERE TYPE = ?)", model.ShieldQuestionCategory)
		if err != nil {
			return err
		}
		known := make(map[string]bool, len(questions))
		for _, question := range questions {
			known[question.ID] = true
		}
		userAnswers, err := listRows[model.UserAnswer](ctx, tx, userAnswersTable, "SHIELD_ID = ?", shieldID)
		if err != nil {
			return err
		}
		answerByQuestion := make(map[string]model.UserAnswer, len(userAnswers))
		for _, userAnswer := range userAnswers {
			answerByQuestion[userAnswer.QuestionID] = userAnswer
		}
		existing, err := listRows[model.Evidence](ctx, tx, evidencesTable, "SHIELD_ID = ?", shieldID)
		if err != nil {
			return err
		}
		evidenceByAnswer := make(map[string]model.Evidence, len(existing))
		for _, evidence := range existing {
			evidenceByAnswer[evidence.UserAnswerID] = evidence
		}
		seen := make(map[string]bool, len(submissions))
		for _, submission := range submissions {
			if seen[submission.QuestionID] {
				return fmt.Errorf("%w: question %q has evidence more than once", answers.ErrInvalid, submission.QuestionID)
			}
			seen[submission.QuestionID] = true
			if !known[submission.QuestionID] {
				return fmt.Errorf("%w: %q is not a shield question", answers.ErrInvalid, submission.QuestionID)
			}
			userAnswer, ok := answerByQuestion[submission.QuestionID]
			if !ok {
				return fmt.Errorf("%w: question %q has no answer to attach evidence to", answers.ErrInvalid, submission.QuestionID)
			}
			selections, err := listRows[model.UserAnswerSelection](ctx, tx, userSelectionsTable, "USER_ANSWER_ID = ?", userAnswer.ID)
			if err != nil {
				return err
			}
			if len(selections) == 0 {
				return fmt.Errorf("%w: question %q has no answer to attach evidence to", answers.ErrInvalid, submission.QuestionID)
			}
			evidence, ok := evidenceByAnswer[userAnswer.ID]
			if !ok {
				shieldIDCopy := shieldID
				evidence = model.Evidence{
					ID: userAnswer.ID + "-evidence", QuestionID: submission.QuestionID,
					UserAnswerID: userAnswer.ID, ShieldID: &shieldIDCopy,
					Description: submission.Description, URL: submission.URL,
					CreatedBy: actor, UpdatedBy: actor,
				}
				if err := insertRow(ctx, tx, evidencesTable, evidence); err != nil {
					return err
				}
				continue
			}
			evidence.Description = submission.Description
			evidence.URL = submission.URL
			evidence.UpdatedBy = actor
			if err := updateRow(ctx, tx, evidencesTable, evidence); err != nil {
				return err
			}
		}
		return nil
	})
}

func launchIsTerminal(state model.LaunchState) bool {
	return state == model.TierValidatedLaunchState || state == model.TimedOutLaunchState || state == model.CancelledLaunchState
}

func shieldIsTerminal(state model.ShieldState) bool {
	return state == model.CompletedShieldState || state == model.TimedOutShieldState || state == model.CancelledShieldState
}

func actorOrDefault(actor, fallback string) string {
	if actor == "" {
		return fallback
	}
	return actor
}
