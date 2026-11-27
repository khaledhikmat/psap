package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/answers"
	"github.com/khaledhikmat/psap/service/data"
)

func (c *Controller) listLaunches(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveLaunches(ctx)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) showLaunch(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveLaunchByID(ctx, ctx.Param("id"))
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			newError(ctx, http.StatusNotFound, err)
			return
		}
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listLaunchAnswers(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveUserAnswersByLaunchID(ctx, ctx.Param("id"))
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listLaunchAnswerHistory(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveUserAnswerHistoryByLaunchID(ctx, ctx.Param("id"))
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

type initiateLaunchRequest struct {
	AssetID string `json:"assetId" binding:"required"`
}

func (c *Controller) initiateLaunch(ctx *gin.Context) {
	var req initiateLaunchRequest
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
		newError(ctx, http.StatusBadRequest, errors.New("asset needs a business owner and manager before it can be launched"))
		return
	}

	launchID, err := newRecordID("launch-")
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	launch := model.Launch{
		ID:        launchID,
		AssetID:   asset.ID,
		State:     model.WaitingForAnswersLaunchState,
		CreatedBy: "launch-api",
		UpdatedBy: "launch-api",
	}
	if err := c.DataSvc.InitiateLaunch(ctx, launch); err != nil {
		writeDataError(ctx, err)
		return
	}
	ctx.JSON(http.StatusAccepted, gin.H{"message": "launch initiated"})
}

type submitLaunchAnswersRequest struct {
	Answers []model.AnswerSubmission `json:"answers" binding:"required"`
}

func (c *Controller) submitLaunchAnswers(ctx *gin.Context) {
	launch, ok := c.loadLaunch(ctx, ctx.Param("id"))
	if !ok {
		return
	}
	if !expectLaunchState(ctx, launch, model.WaitingForAnswersLaunchState) {
		return
	}
	var req submitLaunchAnswersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}
	asset, err := c.DataSvc.RetrieveAssetByID(ctx, launch.AssetID)
	if err != nil {
		writeDataError(ctx, err)
		return
	}
	if err := c.validateAnswerSubmissions(ctx, model.LaunchQuestionCategory, req.Answers); err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	actor := "launch-api"
	if asset.BusinessOwnerID != nil {
		actor = *asset.BusinessOwnerID
	}
	if err := c.DataSvc.SaveLaunchAnswers(ctx, launch.ID, req.Answers, actor); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "answers submitted"})
}

type finalizeLaunchAnswersRequest struct {
	Answers []model.AnswerSubmission `json:"answers"`
}

func (c *Controller) finalizeLaunchAnswers(ctx *gin.Context) {
	launch, ok := c.loadLaunch(ctx, ctx.Param("id"))
	if !ok {
		return
	}
	if !expectLaunchState(ctx, launch, model.WaitingForAnswersLaunchState) {
		return
	}
	var req finalizeLaunchAnswersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}
	asset, err := c.DataSvc.RetrieveAssetByID(ctx, launch.AssetID)
	if err != nil {
		writeDataError(ctx, err)
		return
	}
	if err := c.validateAnswerSubmissions(ctx, model.LaunchQuestionCategory, req.Answers); err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	actor := "launch-api"
	if asset.BusinessOwnerID != nil {
		actor = *asset.BusinessOwnerID
	}
	if err := c.DataSvc.SaveLaunchAnswers(ctx, launch.ID, req.Answers, actor); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	tierID, err := c.calculateTier(ctx, launch.ID, model.LaunchQuestionCategory)
	if err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	launch.TierID = &tierID
	launch.State = model.WaitingForTierApprovalLaunchState
	launch.Reminders, launch.Warnings, launch.Escalations = 0, 0, 0
	launch.UpdatedBy = "launch-api"
	if err := c.DataSvc.UpdateLaunch(ctx, launch); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "answers finalized"})
}

type reviseLaunchAnswersRequest struct {
	Answers []model.AnswerSubmission `json:"answers" binding:"required"`
}

func (c *Controller) reviseLaunchAnswers(ctx *gin.Context) {
	launch, ok := c.loadLaunch(ctx, ctx.Param("id"))
	if !ok {
		return
	}
	if !expectLaunchState(ctx, launch, model.WaitingForTierApprovalLaunchState) {
		return
	}
	var req reviseLaunchAnswersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}
	if err := c.validateAnswerSubmissions(ctx, model.LaunchQuestionCategory, req.Answers); err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	asset, err := c.DataSvc.RetrieveAssetByID(ctx, launch.AssetID)
	if err != nil {
		writeDataError(ctx, err)
		return
	}
	actor := "launch-api"
	if asset.BusinessManagerID != nil {
		actor = *asset.BusinessManagerID
	}
	if err := c.DataSvc.SaveLaunchAnswers(ctx, launch.ID, req.Answers, actor); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	tierID, err := c.calculateTier(ctx, launch.ID, model.LaunchQuestionCategory)
	if err != nil {
		writeSubmissionError(ctx, err)
		return
	}
	launch.TierID = &tierID
	launch.UpdatedBy = "launch-api"
	if err := c.DataSvc.UpdateLaunch(ctx, launch); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "answers revised", "tierId": tierID})
}

type validateLaunchTierRequest struct {
	TierID string `json:"tierId"`
}

func (c *Controller) validateLaunchTier(ctx *gin.Context) {
	launch, ok := c.loadLaunch(ctx, ctx.Param("id"))
	if !ok {
		return
	}
	if !expectLaunchState(ctx, launch, model.WaitingForTierApprovalLaunchState) {
		return
	}
	var req validateLaunchTierRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}
	if req.TierID != "" {
		launch.TierID = &req.TierID
	}
	if launch.TierID == nil || *launch.TierID == "" {
		newError(ctx, http.StatusConflict, errors.New("launch has no calculated tier to validate"))
		return
	}
	if err := c.DataSvc.ValidateLaunchTier(ctx, launch.ID, req.TierID, "launch-api"); err != nil {
		switch {
		case errors.Is(err, data.ErrNotFound):
			newError(ctx, http.StatusBadRequest, fmt.Errorf("unknown tier %q", req.TierID))
		case errors.Is(err, data.ErrInvalidState):
			newError(ctx, http.StatusConflict, err)
		default:
			newError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	asset, err := c.DataSvc.RetrieveAssetByID(ctx, launch.AssetID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	if err := c.createShieldForAsset(ctx, asset); err != nil {
		newError(ctx, http.StatusInternalServerError, fmt.Errorf("tier validated but shield initiation failed: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "tier validated"})
}

func (c *Controller) loadLaunch(ctx *gin.Context, id string) (model.Launch, bool) {
	launch, err := c.DataSvc.RetrieveLaunchByID(ctx, id)
	if err != nil {
		writeDataError(ctx, err)
		return model.Launch{}, false
	}
	return launch, true
}

func expectLaunchState(ctx *gin.Context, launch model.Launch, want model.LaunchState) bool {
	if launch.State == want {
		return true
	}
	newError(ctx, http.StatusConflict, fmt.Errorf("launch is %s, want %s", launch.State, want))
	return false
}

func (c *Controller) validateAnswerSubmissions(ctx *gin.Context, category model.QuestionCategoryType, submissions []model.AnswerSubmission) error {
	questions, err := c.DataSvc.RetrieveQuestionsByCategoryType(ctx, category)
	if err != nil {
		return err
	}
	predefined, err := c.DataSvc.RetrievePredefinedAnswersByCategoryType(ctx, category)
	if err != nil {
		return err
	}
	return answers.Validate(questions, predefined, submissions)
}

func (c *Controller) calculateTier(ctx *gin.Context, launchID string, category model.QuestionCategoryType) (string, error) {
	saved, err := c.DataSvc.RetrieveUserAnswersByLaunchID(ctx, launchID)
	if err != nil {
		return "", err
	}
	predefined, err := c.DataSvc.RetrievePredefinedAnswersByCategoryType(ctx, category)
	if err != nil {
		return "", err
	}
	tier, err := c.DataSvc.RetrieveTierByPoints(ctx, answers.Points(saved, predefined))
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return "", fmt.Errorf("no tier covers calculated points: %w", err)
		}
		return "", err
	}
	return tier.ID, nil
}
