package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/answers"
)

func (c *Controller) listShields(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveShields(ctx)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) showShield(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveShieldByID(ctx, ctx.Param("id"))
	if err != nil {
		writeDataError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listShieldAnswers(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveUserAnswersByShieldID(ctx, ctx.Param("id"))
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listShieldEvidences(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveEvidencesByShieldID(ctx, ctx.Param("id"))
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listShieldAnswerHistory(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveUserAnswerHistoryByShieldID(ctx, ctx.Param("id"))
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

type initiateShieldRequest struct {
	AssetID string `json:"assetId" binding:"required"`
}

func (c *Controller) initiateShield(ctx *gin.Context) {
	var req initiateShieldRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}
	asset, err := c.DataSvc.RetrieveAssetByID(ctx, req.AssetID)
	if err != nil {
		writeDataError(ctx, err)
		return
	}
	if asset.BusinessOwnerID == nil || asset.BusinessManagerID == nil {
		newError(ctx, http.StatusBadRequest, errors.New("asset needs a business owner and manager before it can be shielded"))
		return
	}
	if err := c.createShieldForAsset(ctx, asset); err != nil {
		writeDataError(ctx, err)
		return
	}
	ctx.JSON(http.StatusAccepted, gin.H{"message": "shield initiated"})
}

type submitShieldAnswersRequest struct {
	Answers []model.AnswerSubmission `json:"answers" binding:"required"`
}

func (c *Controller) submitShieldAnswers(ctx *gin.Context) {
	shield, ok := c.loadShield(ctx, ctx.Param("id"))
	if !ok {
		return
	}
	if !expectShieldState(ctx, shield, model.WaitingForAnswersShieldState) {
		return
	}
	var req submitShieldAnswersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}
	if err := c.validateAnswerSubmissions(ctx, model.ShieldQuestionCategory, req.Answers); err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	actor, err := c.shieldActor(ctx, shield.AssetID)
	if err != nil {
		writeDataError(ctx, err)
		return
	}
	if err := c.DataSvc.SaveShieldAnswers(ctx, shield.ID, req.Answers, actor); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "answers submitted"})
}

type finalizeShieldAnswersRequest struct {
	Answers []model.AnswerSubmission `json:"answers"`
}

func (c *Controller) finalizeShieldAnswers(ctx *gin.Context) {
	shield, ok := c.loadShield(ctx, ctx.Param("id"))
	if !ok {
		return
	}
	if !expectShieldState(ctx, shield, model.WaitingForAnswersShieldState) {
		return
	}
	var req finalizeShieldAnswersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}
	if err := c.validateAnswerSubmissions(ctx, model.ShieldQuestionCategory, req.Answers); err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	questions, err := c.DataSvc.RetrieveQuestionsByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	stored, err := c.DataSvc.RetrieveUserAnswersByShieldID(ctx, shield.ID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	if err := validateAllQuestionsAnswered(questions, stored, req.Answers); err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	actor, err := c.shieldActor(ctx, shield.AssetID)
	if err != nil {
		writeDataError(ctx, err)
		return
	}
	if err := c.DataSvc.SaveShieldAnswers(ctx, shield.ID, req.Answers, actor); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	shield.State = model.WaitingForEvidencesShieldState
	shield.UpdatedBy = actor
	if err := c.DataSvc.UpdateShield(ctx, shield); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "answers finalized"})
}

type submitShieldEvidencesRequest struct {
	Evidences []model.EvidenceSubmission `json:"evidences" binding:"required"`
}

func (c *Controller) submitShieldEvidences(ctx *gin.Context) {
	shield, ok := c.loadShield(ctx, ctx.Param("id"))
	if !ok {
		return
	}
	if !expectShieldState(ctx, shield, model.WaitingForEvidencesShieldState) {
		return
	}
	var req submitShieldEvidencesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}
	actor, err := c.shieldActor(ctx, shield.AssetID)
	if err != nil {
		writeDataError(ctx, err)
		return
	}
	if err := c.saveShieldEvidences(ctx, shield.ID, req.Evidences, actor); err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "evidences submitted"})
}

type finalizeShieldEvidencesRequest struct {
	Evidences []model.EvidenceSubmission `json:"evidences"`
}

func (c *Controller) finalizeShieldEvidences(ctx *gin.Context) {
	shield, ok := c.loadShield(ctx, ctx.Param("id"))
	if !ok {
		return
	}
	if !expectShieldState(ctx, shield, model.WaitingForEvidencesShieldState) {
		return
	}
	var req finalizeShieldEvidencesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}
	actor, err := c.shieldActor(ctx, shield.AssetID)
	if err != nil {
		writeDataError(ctx, err)
		return
	}
	questions, err := c.DataSvc.RetrieveQuestionsByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	existing, err := c.DataSvc.RetrieveEvidencesByShieldID(ctx, shield.ID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	if err := validateRequiredEvidences(questions, mergeEvidenceSubmissions(existing, req.Evidences)); err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	if err := c.saveShieldEvidences(ctx, shield.ID, req.Evidences, actor); err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	evidences, err := c.DataSvc.RetrieveEvidencesByShieldID(ctx, shield.ID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	if err := validateRequiredEvidences(questions, evidences); err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	userAnswers, err := c.DataSvc.RetrieveUserAnswersByShieldID(ctx, shield.ID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	predefined, err := c.DataSvc.RetrievePredefinedAnswersByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	if err := c.DataSvc.CompleteShield(ctx, shield.ID, answers.Points(userAnswers, predefined), actor); err != nil {
		writeDataError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "evidences finalized"})
}

func (c *Controller) loadShield(ctx *gin.Context, id string) (model.Shield, bool) {
	shield, err := c.DataSvc.RetrieveShieldByID(ctx, id)
	if err != nil {
		writeDataError(ctx, err)
		return model.Shield{}, false
	}
	return shield, true
}

func expectShieldState(ctx *gin.Context, shield model.Shield, want model.ShieldState) bool {
	if shield.State == want {
		return true
	}
	newError(ctx, http.StatusConflict, fmt.Errorf("shield is %s, want %s", shield.State, want))
	return false
}

func (c *Controller) shieldActor(ctx *gin.Context, assetID string) (string, error) {
	asset, err := c.DataSvc.RetrieveAssetByID(ctx, assetID)
	if err != nil {
		return "", err
	}
	if asset.BusinessManagerID == nil {
		return "shield-api", nil
	}
	return *asset.BusinessManagerID, nil
}

func (c *Controller) saveShieldEvidences(ctx *gin.Context, shieldID string, submissions []model.EvidenceSubmission, actor string) error {
	return c.DataSvc.SaveShieldEvidences(ctx, shieldID, submissions, actor)
}

func validateAllQuestionsAnswered(questions []model.Question, stored []model.UserAnswerDetail, final []model.AnswerSubmission) error {
	answered := make(map[string]bool, len(questions))
	for _, userAnswer := range stored {
		answered[userAnswer.QuestionID] = len(userAnswer.Selections) > 0
	}
	for _, submission := range final {
		answered[submission.QuestionID] = len(submission.PredefinedAnswerIDs) > 0
	}
	missing := []string{}
	for _, question := range questions {
		if !answered[question.ID] {
			missing = append(missing, question.ID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: unanswered questions: %s", errIncompleteShield, strings.Join(missing, ", "))
	}
	return nil
}

func validateRequiredEvidences(questions []model.Question, evidences []model.Evidence) error {
	provided := make(map[string]bool, len(evidences))
	for _, evidence := range evidences {
		provided[evidence.QuestionID] = evidence.Description != "" || evidence.URL != ""
	}
	missing := []string{}
	for _, question := range questions {
		if question.EvidenceRequired && !provided[question.ID] {
			missing = append(missing, question.ID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing evidence for: %s", errIncompleteShield, strings.Join(missing, ", "))
	}
	return nil
}

func mergeEvidenceSubmissions(existing []model.Evidence, submissions []model.EvidenceSubmission) []model.Evidence {
	merged := make(map[string]model.Evidence, len(existing)+len(submissions))
	for _, evidence := range existing {
		merged[evidence.QuestionID] = evidence
	}
	for _, submission := range submissions {
		merged[submission.QuestionID] = model.Evidence{
			QuestionID:  submission.QuestionID,
			Description: submission.Description,
			URL:         submission.URL,
		}
	}
	result := make([]model.Evidence, 0, len(merged))
	for _, evidence := range merged {
		result = append(result, evidence)
	}
	return result
}
