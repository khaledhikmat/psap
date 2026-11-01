package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/data"
)

// List and Get

func (c *Controller) listLaunches(ctx *gin.Context) {
	result, err := c.Svcs.DataSvc.RetrieveLaunches(ctx)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) showLaunch(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveLaunchByID(ctx, id)
	if err != nil {
		if err == data.ErrNotFound {
			newError(ctx, http.StatusNotFound, err)
			return
		}
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listLaunchAnswers(ctx *gin.Context) {
	launchID := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveUserAnswersByLaunchID(ctx, launchID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listLaunchAnswerHistory(ctx *gin.Context) {
	launchID := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveUserAnswerHistoryByLaunchID(ctx, launchID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

// Workflow Operations

type initiateLaunchRequest struct {
	AssetID string `json:"assetId" binding:"required"`
}

func (c *Controller) initiateLaunch(ctx *gin.Context) {
	var req initiateLaunchRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	if err := c.Svcs.FlowSvc.InitiateLaunch(ctx, req.AssetID); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusAccepted, gin.H{"message": "launch initiated"})
}

type submitLaunchAnswersRequest struct {
	Answers []model.AnswerSubmission `json:"answers" binding:"required"`
}

func (c *Controller) submitLaunchAnswers(ctx *gin.Context) {
	launchID := ctx.Param("id")

	var req submitLaunchAnswersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	// Get the launch to find the asset ID
	launch, err := c.Svcs.DataSvc.RetrieveLaunchByID(ctx, launchID)
	if err != nil {
		if err == data.ErrNotFound {
			newError(ctx, http.StatusNotFound, err)
			return
		}
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := c.Svcs.FlowSvc.SubmitLaunchAnswers(ctx, launch.AssetID, req.Answers); err != nil {
		newError(ctx, submissionErrorStatus(err), err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "answers submitted"})
}

type finalizeLaunchAnswersRequest struct {
	Answers []model.AnswerSubmission `json:"answers"`
}

func (c *Controller) finalizeLaunchAnswers(ctx *gin.Context) {
	launchID := ctx.Param("id")

	var req finalizeLaunchAnswersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	// Get the launch to find the asset ID
	launch, err := c.Svcs.DataSvc.RetrieveLaunchByID(ctx, launchID)
	if err != nil {
		if err == data.ErrNotFound {
			newError(ctx, http.StatusNotFound, err)
			return
		}
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := c.Svcs.FlowSvc.FinalizeLaunchAnswers(ctx, launch.AssetID, req.Answers); err != nil {
		newError(ctx, submissionErrorStatus(err), err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "answers finalized"})
}

type validateLaunchTierRequest struct {
	TierID string `json:"tierId"`
}

func (c *Controller) validateLaunchTier(ctx *gin.Context) {
	launchID := ctx.Param("id")

	var req validateLaunchTierRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	// Get the launch to find the asset ID
	launch, err := c.Svcs.DataSvc.RetrieveLaunchByID(ctx, launchID)
	if err != nil {
		if err == data.ErrNotFound {
			newError(ctx, http.StatusNotFound, err)
			return
		}
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := c.Svcs.FlowSvc.ValidateTier(ctx, launch.AssetID, req.TierID); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "tier validated"})
}
