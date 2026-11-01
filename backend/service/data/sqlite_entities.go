package data

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/khaledhikmat/psap/model"
)

// Workers

func (s *sqliteService) NewWorker(ctx context.Context, worker model.Worker) error {
	return insertRow(ctx, s.db, workersTable, worker)
}

func (s *sqliteService) RetrieveWorkers(ctx context.Context) ([]model.Worker, error) {
	return listRows[model.Worker](ctx, s.db, workersTable, "")
}

func (s *sqliteService) RetrieveWorkerByID(ctx context.Context, id string) (model.Worker, error) {
	return getRow[model.Worker](ctx, s.db, workersTable, "ID = ?", id)
}

func (s *sqliteService) UpdateWorker(ctx context.Context, worker model.Worker) error {
	return updateRow(ctx, s.db, workersTable, worker)
}

// Assets

func (s *sqliteService) NewAsset(ctx context.Context, asset model.Asset) error {
	return insertRow(ctx, s.db, assetsTable, asset)
}

func (s *sqliteService) RetrieveAssets(ctx context.Context) ([]model.Asset, error) {
	return listRows[model.Asset](ctx, s.db, assetsTable, "")
}

func (s *sqliteService) RetrieveAssetByID(ctx context.Context, id string) (model.Asset, error) {
	return getRow[model.Asset](ctx, s.db, assetsTable, "ASSET_ID = ?", id)
}

func (s *sqliteService) RetrieveAssetByName(ctx context.Context, name string) (model.Asset, error) {
	return getRow[model.Asset](ctx, s.db, assetsTable, "ASSET_NAME = ?", name)
}

func (s *sqliteService) UpdateAsset(ctx context.Context, asset model.Asset) error {
	return updateRow(ctx, s.db, assetsTable, asset)
}

// Question categories

func (s *sqliteService) NewQuestionCategory(ctx context.Context, category model.QuestionCategory) error {
	return insertRow(ctx, s.db, questionCategoriesTable, category)
}

func (s *sqliteService) RetrieveQuestionCategories(ctx context.Context) ([]model.QuestionCategory, error) {
	return listRows[model.QuestionCategory](ctx, s.db, questionCategoriesTable, "")
}

func (s *sqliteService) RetrieveQuestionCategoriesByType(ctx context.Context, categoryType model.QuestionCategoryType) ([]model.QuestionCategory, error) {
	return listRows[model.QuestionCategory](ctx, s.db, questionCategoriesTable, "TYPE = ?", categoryType)
}

func (s *sqliteService) RetrieveQuestionCategoryByID(ctx context.Context, id string) (model.QuestionCategory, error) {
	return getRow[model.QuestionCategory](ctx, s.db, questionCategoriesTable, "ID = ?", id)
}

func (s *sqliteService) UpdateQuestionCategory(ctx context.Context, category model.QuestionCategory) error {
	return updateRow(ctx, s.db, questionCategoriesTable, category)
}

// Questions

func (s *sqliteService) NewQuestion(ctx context.Context, question model.Question) error {
	return insertRow(ctx, s.db, questionsTable, question)
}

func (s *sqliteService) RetrieveQuestions(ctx context.Context) ([]model.Question, error) {
	return listRows[model.Question](ctx, s.db, questionsTable, "")
}

func (s *sqliteService) RetrieveQuestionsByCategoryID(ctx context.Context, categoryID string) ([]model.Question, error) {
	questions := []model.Question{}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE CATEGORY_ID = ? ORDER BY DISPLAY_ORDER, ID", questionsTable.columnList(""), questionsTable.name)
	err := s.db.SelectContext(ctx, &questions, query, categoryID)
	return questions, err
}

// RetrieveQuestionsByCategoryType returns questions in category order, then display order.
func (s *sqliteService) RetrieveQuestionsByCategoryType(ctx context.Context, categoryType model.QuestionCategoryType) ([]model.Question, error) {
	questions := []model.Question{}
	query := fmt.Sprintf(
		"SELECT %s FROM %s q JOIN %s c ON c.ID = q.CATEGORY_ID WHERE c.TYPE = ? ORDER BY c.CREATED_AT, c.ID, q.DISPLAY_ORDER, q.ID",
		questionsTable.columnList("q."), questionsTable.name, questionCategoriesTable.name)
	err := s.db.SelectContext(ctx, &questions, query, categoryType)
	return questions, err
}

func (s *sqliteService) RetrieveQuestionByID(ctx context.Context, id string) (model.Question, error) {
	return getRow[model.Question](ctx, s.db, questionsTable, "ID = ?", id)
}

func (s *sqliteService) UpdateQuestion(ctx context.Context, question model.Question) error {
	return updateRow(ctx, s.db, questionsTable, question)
}

// Predefined answers

func (s *sqliteService) NewPredefinedAnswer(ctx context.Context, answer model.PredefinedAnswer) error {
	return insertRow(ctx, s.db, predefinedAnswersTable, answer)
}

func (s *sqliteService) RetrievePredefinedAnswers(ctx context.Context) ([]model.PredefinedAnswer, error) {
	return listRows[model.PredefinedAnswer](ctx, s.db, predefinedAnswersTable, "")
}

func (s *sqliteService) RetrievePredefinedAnswersByQuestionID(ctx context.Context, questionID string) ([]model.PredefinedAnswer, error) {
	answers := []model.PredefinedAnswer{}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE QUESTION_ID = ? ORDER BY DISPLAY_ORDER, ID",
		predefinedAnswersTable.columnList(""), predefinedAnswersTable.name)
	err := s.db.SelectContext(ctx, &answers, query, questionID)
	return answers, err
}

func (s *sqliteService) RetrievePredefinedAnswersByCategoryType(ctx context.Context, categoryType model.QuestionCategoryType) ([]model.PredefinedAnswer, error) {
	answers := []model.PredefinedAnswer{}
	query := fmt.Sprintf(
		"SELECT %s FROM %s p JOIN %s q ON q.ID = p.QUESTION_ID JOIN %s c ON c.ID = q.CATEGORY_ID WHERE c.TYPE = ? "+
			"ORDER BY c.CREATED_AT, c.ID, q.DISPLAY_ORDER, q.ID, p.DISPLAY_ORDER, p.ID",
		predefinedAnswersTable.columnList("p."), predefinedAnswersTable.name, questionsTable.name, questionCategoriesTable.name)
	err := s.db.SelectContext(ctx, &answers, query, categoryType)
	return answers, err
}

func (s *sqliteService) RetrievePredefinedAnswerByID(ctx context.Context, id string) (model.PredefinedAnswer, error) {
	return getRow[model.PredefinedAnswer](ctx, s.db, predefinedAnswersTable, "ID = ?", id)
}

func (s *sqliteService) UpdatePredefinedAnswer(ctx context.Context, answer model.PredefinedAnswer) error {
	return updateRow(ctx, s.db, predefinedAnswersTable, answer)
}

// User answers

func (s *sqliteService) SaveLaunchAnswers(ctx context.Context, launchID string, submissions []model.AnswerSubmission, actor string) error {
	return s.saveUserAnswers(ctx, "LAUNCH_ID", launchID, submissions, actor)
}

func (s *sqliteService) SaveShieldAnswers(ctx context.Context, shieldID string, submissions []model.AnswerSubmission, actor string) error {
	return s.saveUserAnswers(ctx, "SHIELD_ID", shieldID, submissions, actor)
}

// saveUserAnswers applies the submissions to the launch or shield (scopeCol) identified by
// scopeID. User answer and selection IDs derive from the scope and question, so repeating a
// submission that already succeeded changes nothing.
func (s *sqliteService) saveUserAnswers(ctx context.Context, scopeCol, scopeID string, submissions []model.AnswerSubmission, actor string) error {
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		existing, err := listRows[model.UserAnswer](ctx, tx, userAnswersTable, scopeCol+" = ?", scopeID)
		if err != nil {
			return err
		}
		byQuestion := make(map[string]model.UserAnswer, len(existing))
		for _, ua := range existing {
			byQuestion[ua.QuestionID] = ua
		}

		for _, sub := range submissions {
			ua, found := byQuestion[sub.QuestionID]
			var previous []string
			if found {
				selections, err := listRows[model.UserAnswerSelection](ctx, tx, userSelectionsTable, "USER_ANSWER_ID = ?", ua.ID)
				if err != nil {
					return err
				}
				for _, sel := range selections {
					previous = append(previous, sel.PredefinedAnswerID)
				}
			}
			added, removed := diffIDs(previous, sub.PredefinedAnswerIDs)
			if found && len(added) == 0 && len(removed) == 0 {
				continue
			}

			if found {
				ua.SubmittedBy = actor
				ua.UpdatedBy = actor
				if err := updateRow(ctx, tx, userAnswersTable, ua); err != nil {
					return err
				}
			} else {
				ua = model.UserAnswer{
					ID:          scopeID + "-" + sub.QuestionID,
					QuestionID:  sub.QuestionID,
					SubmittedBy: actor,
					CreatedBy:   actor,
					UpdatedBy:   actor,
				}
				if scopeCol == "LAUNCH_ID" {
					ua.LaunchID = &scopeID
				} else {
					ua.ShieldID = &scopeID
				}
				if err := insertRow(ctx, tx, userAnswersTable, ua); err != nil {
					return err
				}
			}

			for _, id := range removed {
				if _, err := tx.ExecContext(ctx, "DELETE FROM "+userSelectionsTable.name+" WHERE USER_ANSWER_ID = ? AND PREDEFINED_ANSWER_ID = ?", ua.ID, id); err != nil {
					return err
				}
			}
			for _, id := range added {
				sel := model.UserAnswerSelection{ID: ua.ID + "-" + id, UserAnswerID: ua.ID, PredefinedAnswerID: id, CreatedBy: actor}
				if err := insertRow(ctx, tx, userSelectionsTable, sel); err != nil {
					return err
				}
			}

			// One history row per change: removed answers are paired with added ones, and a
			// removal without a replacement records an empty new answer.
			changedAt := time.Now().UTC()
			for i := 0; i < max(len(added), len(removed)); i++ {
				h := model.UserAnswerHistory{
					ID:           fmt.Sprintf("%s-%s-%d", ua.ID, changedAt.Format("20060102150405.000000"), i),
					UserAnswerID: ua.ID,
					QuestionID:   ua.QuestionID,
					LaunchID:     ua.LaunchID,
					ShieldID:     ua.ShieldID,
					ChangedBy:    actor,
					ChangedAt:    changedAt.Format(time.RFC3339Nano),
				}
				if i < len(removed) {
					h.PreviousPredefinedAnswerID = &removed[i]
				}
				if i < len(added) {
					h.NewPredefinedAnswerID = added[i]
				}
				if err := insertRow(ctx, tx, userAnswerHistoryTable, h); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// diffIDs returns the IDs in next that are not in previous (added) and vice versa (removed).
func diffIDs(previous, next []string) (added, removed []string) {
	was := make(map[string]bool, len(previous))
	for _, id := range previous {
		was[id] = true
	}
	is := make(map[string]bool, len(next))
	for _, id := range next {
		is[id] = true
		if !was[id] {
			added = append(added, id)
		}
	}
	for _, id := range previous {
		if !is[id] {
			removed = append(removed, id)
		}
	}
	return added, removed
}

func (s *sqliteService) RetrieveUserAnswersByLaunchID(ctx context.Context, launchID string) ([]model.UserAnswerDetail, error) {
	return s.userAnswerDetails(ctx, "LAUNCH_ID", launchID)
}

func (s *sqliteService) RetrieveUserAnswersByShieldID(ctx context.Context, shieldID string) ([]model.UserAnswerDetail, error) {
	return s.userAnswerDetails(ctx, "SHIELD_ID", shieldID)
}

func (s *sqliteService) RetrieveUserAnswerByID(ctx context.Context, id string) (model.UserAnswerDetail, error) {
	answers, err := s.userAnswerDetails(ctx, "ID", id)
	if err != nil {
		return model.UserAnswerDetail{}, err
	}
	if len(answers) == 0 {
		return model.UserAnswerDetail{}, fmt.Errorf("%s %v: %w", userAnswersTable.name, []any{id}, ErrNotFound)
	}
	return answers[0], nil
}

// userAnswerDetails returns the user answers whose col equals val, each with its selections.
func (s *sqliteService) userAnswerDetails(ctx context.Context, col, val string) ([]model.UserAnswerDetail, error) {
	answers, err := listRows[model.UserAnswer](ctx, s.db, userAnswersTable, col+" = ?", val)
	if err != nil {
		return nil, err
	}

	selections := []model.UserAnswerSelection{}
	query := fmt.Sprintf("SELECT %s FROM %s s JOIN %s a ON a.ID = s.USER_ANSWER_ID WHERE a.%s = ? ORDER BY s.CREATED_AT, s.ID",
		userSelectionsTable.columnList("s."), userSelectionsTable.name, userAnswersTable.name, col)
	if err := s.db.SelectContext(ctx, &selections, query, val); err != nil {
		return nil, err
	}
	byAnswer := make(map[string][]model.UserAnswerSelection, len(answers))
	for _, sel := range selections {
		byAnswer[sel.UserAnswerID] = append(byAnswer[sel.UserAnswerID], sel)
	}

	details := make([]model.UserAnswerDetail, len(answers))
	for i, ua := range answers {
		details[i] = model.UserAnswerDetail{UserAnswer: ua, Selections: byAnswer[ua.ID]}
		if details[i].Selections == nil {
			details[i].Selections = []model.UserAnswerSelection{}
		}
	}
	return details, nil
}

// User answer history

func (s *sqliteService) RetrieveUserAnswerHistoryByUserAnswerID(ctx context.Context, userAnswerID string) ([]model.UserAnswerHistory, error) {
	return s.userAnswerHistory(ctx, "USER_ANSWER_ID", userAnswerID)
}

func (s *sqliteService) RetrieveUserAnswerHistoryByLaunchID(ctx context.Context, launchID string) ([]model.UserAnswerHistory, error) {
	return s.userAnswerHistory(ctx, "LAUNCH_ID", launchID)
}

func (s *sqliteService) RetrieveUserAnswerHistoryByShieldID(ctx context.Context, shieldID string) ([]model.UserAnswerHistory, error) {
	return s.userAnswerHistory(ctx, "SHIELD_ID", shieldID)
}

func (s *sqliteService) userAnswerHistory(ctx context.Context, col, val string) ([]model.UserAnswerHistory, error) {
	history := []model.UserAnswerHistory{}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ? ORDER BY CHANGED_AT DESC, ID DESC",
		userAnswerHistoryTable.columnList(""), userAnswerHistoryTable.name, col)
	err := s.db.SelectContext(ctx, &history, query, val)
	return history, err
}

// Launches

func (s *sqliteService) NewLaunch(ctx context.Context, launch model.Launch) error {
	return insertRow(ctx, s.db, launchesTable, launch)
}

func (s *sqliteService) RetrieveLaunches(ctx context.Context) ([]model.Launch, error) {
	return listRows[model.Launch](ctx, s.db, launchesTable, "")
}

func (s *sqliteService) RetrieveLaunchesByAssetID(ctx context.Context, assetID string) ([]model.Launch, error) {
	return listRows[model.Launch](ctx, s.db, launchesTable, "ASSET_ID = ?", assetID)
}

func (s *sqliteService) RetrieveLaunchByID(ctx context.Context, id string) (model.Launch, error) {
	return getRow[model.Launch](ctx, s.db, launchesTable, "ID = ?", id)
}

func (s *sqliteService) UpdateLaunch(ctx context.Context, launch model.Launch) error {
	return updateRow(ctx, s.db, launchesTable, launch)
}

// Shields

func (s *sqliteService) NewShield(ctx context.Context, shield model.Shield) error {
	return insertRow(ctx, s.db, shieldsTable, shield)
}

func (s *sqliteService) RetrieveShields(ctx context.Context) ([]model.Shield, error) {
	return listRows[model.Shield](ctx, s.db, shieldsTable, "")
}

func (s *sqliteService) RetrieveShieldsByAssetID(ctx context.Context, assetID string) ([]model.Shield, error) {
	return listRows[model.Shield](ctx, s.db, shieldsTable, "ASSET_ID = ?", assetID)
}

func (s *sqliteService) RetrieveShieldByID(ctx context.Context, id string) (model.Shield, error) {
	return getRow[model.Shield](ctx, s.db, shieldsTable, "ID = ?", id)
}

func (s *sqliteService) UpdateShield(ctx context.Context, shield model.Shield) error {
	return updateRow(ctx, s.db, shieldsTable, shield)
}

// Evidences

func (s *sqliteService) NewEvidences(ctx context.Context, evidences []model.Evidence) error {
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		for _, e := range evidences {
			if err := insertRow(ctx, tx, evidencesTable, e); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *sqliteService) RetrieveEvidencesByLaunchID(ctx context.Context, launchID string) ([]model.Evidence, error) {
	return listRows[model.Evidence](ctx, s.db, evidencesTable, "LAUNCH_ID = ?", launchID)
}

func (s *sqliteService) RetrieveEvidencesByShieldID(ctx context.Context, shieldID string) ([]model.Evidence, error) {
	return listRows[model.Evidence](ctx, s.db, evidencesTable, "SHIELD_ID = ?", shieldID)
}

func (s *sqliteService) RetrieveEvidenceByID(ctx context.Context, id string) (model.Evidence, error) {
	return getRow[model.Evidence](ctx, s.db, evidencesTable, "ID = ?", id)
}

func (s *sqliteService) UpdateEvidences(ctx context.Context, evidences []model.Evidence) error {
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		for _, e := range evidences {
			if err := updateRow(ctx, tx, evidencesTable, e); err != nil {
				return err
			}
		}
		return nil
	})
}

// Baselines

func (s *sqliteService) NewBaseline(ctx context.Context, baseline model.Baseline) error {
	return insertRow(ctx, s.db, baselinesTable, baseline)
}

func (s *sqliteService) RetrieveBaselines(ctx context.Context) ([]model.Baseline, error) {
	return listRows[model.Baseline](ctx, s.db, baselinesTable, "")
}

func (s *sqliteService) RetrieveBaselinesByAssetID(ctx context.Context, assetID string) ([]model.Baseline, error) {
	return listRows[model.Baseline](ctx, s.db, baselinesTable, "ASSET_ID = ?", assetID)
}

func (s *sqliteService) RetrieveBaselineByID(ctx context.Context, id string) (model.Baseline, error) {
	return getRow[model.Baseline](ctx, s.db, baselinesTable, "ID = ?", id)
}

func (s *sqliteService) UpdateBaseline(ctx context.Context, baseline model.Baseline) error {
	return updateRow(ctx, s.db, baselinesTable, baseline)
}

// Action items

func (s *sqliteService) NewActionItem(ctx context.Context, actionItem model.ActionItem) error {
	return insertRow(ctx, s.db, actionItemsTable, actionItem)
}

func (s *sqliteService) RetrieveActionItems(ctx context.Context) ([]model.ActionItem, error) {
	return listRows[model.ActionItem](ctx, s.db, actionItemsTable, "")
}

func (s *sqliteService) RetrieveActionItemsByBaselineID(ctx context.Context, baselineID string) ([]model.ActionItem, error) {
	return listRows[model.ActionItem](ctx, s.db, actionItemsTable, "BASELINE_ID = ?", baselineID)
}

func (s *sqliteService) RetrieveActionItemByID(ctx context.Context, id string) (model.ActionItem, error) {
	return getRow[model.ActionItem](ctx, s.db, actionItemsTable, "ID = ?", id)
}

func (s *sqliteService) UpdateActionItem(ctx context.Context, actionItem model.ActionItem) error {
	return updateRow(ctx, s.db, actionItemsTable, actionItem)
}

// Tiers

func (s *sqliteService) NewTier(ctx context.Context, tier model.Tier) error {
	return insertRow(ctx, s.db, tiersTable, tier)
}

func (s *sqliteService) RetrieveTiers(ctx context.Context) ([]model.Tier, error) {
	return listRows[model.Tier](ctx, s.db, tiersTable, "")
}

func (s *sqliteService) RetrieveTierByID(ctx context.Context, id string) (model.Tier, error) {
	return getRow[model.Tier](ctx, s.db, tiersTable, "ID = ?", id)
}

func (s *sqliteService) RetrieveTierByPoints(ctx context.Context, points int) (model.Tier, error) {
	return getRow[model.Tier](ctx, s.db, tiersTable, "FROM_POINT <= ? AND ? <= TO_POINT", points, points)
}

func (s *sqliteService) UpdateTier(ctx context.Context, tier model.Tier) error {
	return updateRow(ctx, s.db, tiersTable, tier)
}
