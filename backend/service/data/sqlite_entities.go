package data

import (
	"context"
	"fmt"

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
	return listRows[model.Question](ctx, s.db, questionsTable, "CATEGORY_ID = ?", categoryID)
}

// RetrieveQuestionsByCategoryType returns questions in category order, then question order.
func (s *sqliteService) RetrieveQuestionsByCategoryType(ctx context.Context, categoryType model.QuestionCategoryType) ([]model.Question, error) {
	questions := []model.Question{}
	query := fmt.Sprintf(
		"SELECT %s FROM %s q JOIN %s c ON c.ID = q.CATEGORY_ID WHERE c.TYPE = ? ORDER BY c.CREATED_AT, c.ID, q.CREATED_AT, q.ID",
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

// Answers

func (s *sqliteService) NewAnswers(ctx context.Context, answers []model.Answer) error {
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		for _, a := range answers {
			if err := insertRow(ctx, tx, answersTable, a); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *sqliteService) RetrieveAnswersByLaunchID(ctx context.Context, launchID string) ([]model.Answer, error) {
	return listRows[model.Answer](ctx, s.db, answersTable, "LAUNCH_ID = ?", launchID)
}

func (s *sqliteService) RetrieveAnswerByID(ctx context.Context, id string) (model.Answer, error) {
	return getRow[model.Answer](ctx, s.db, answersTable, "ID = ?", id)
}

func (s *sqliteService) UpdateAnswers(ctx context.Context, answers []model.Answer) error {
	return s.withTx(ctx, func(tx *sqlx.Tx) error {
		for _, a := range answers {
			if err := updateRow(ctx, tx, answersTable, a); err != nil {
				return err
			}
		}
		return nil
	})
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
