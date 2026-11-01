package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/data"
)

// List and Get

func (c *Controller) listShields(ctx *gin.Context) {
	result, err := c.Svcs.DataSvc.RetrieveShields(ctx)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) showShield(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveShieldByID(ctx, id)
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

func (c *Controller) listShieldAnswers(ctx *gin.Context) {
	shieldID := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveUserAnswersByShieldID(ctx, shieldID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listShieldEvidences(ctx *gin.Context) {
	shieldID := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveEvidencesByShieldID(ctx, shieldID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listShieldAnswerHistory(ctx *gin.Context) {
	shieldID := ctx.Param("id")

	result, err := c.Svcs.DataSvc.RetrieveUserAnswerHistoryByShieldID(ctx, shieldID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

// Workflow Operations

type initiateShieldRequest struct {
	AssetID string `json:"assetId" binding:"required"`
}

func (c *Controller) initiateShield(ctx *gin.Context) {
	var req initiateShieldRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	if err := c.Svcs.FlowSvc.InitiateShield(ctx, req.AssetID); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusAccepted, gin.H{"message": "shield initiated"})
}

type submitShieldAnswersRequest struct {
	Answers []model.AnswerSubmission `json:"answers" binding:"required"`
}

func (c *Controller) submitShieldAnswers(ctx *gin.Context) {
	shieldID := ctx.Param("id")

	var req submitShieldAnswersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	// Get the shield to find the asset ID
	shield, err := c.Svcs.DataSvc.RetrieveShieldByID(ctx, shieldID)
	if err != nil {
		if err == data.ErrNotFound {
			newError(ctx, http.StatusNotFound, err)
			return
		}
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := c.Svcs.FlowSvc.SubmitShieldAnswers(ctx, shield.AssetID, req.Answers); err != nil {
		newError(ctx, submissionErrorStatus(err), err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "answers submitted"})
}

type finalizeShieldAnswersRequest struct {
	Answers []model.AnswerSubmission `json:"answers"`
}

func (c *Controller) finalizeShieldAnswers(ctx *gin.Context) {
	shieldID := ctx.Param("id")

	var req finalizeShieldAnswersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	// Get the shield to find the asset ID
	shield, err := c.Svcs.DataSvc.RetrieveShieldByID(ctx, shieldID)
	if err != nil {
		if err == data.ErrNotFound {
			newError(ctx, http.StatusNotFound, err)
			return
		}
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := c.Svcs.FlowSvc.FinalizeShieldAnswers(ctx, shield.AssetID, req.Answers); err != nil {
		newError(ctx, submissionErrorStatus(err), err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "answers finalized"})
}

type submitShieldEvidencesRequest struct {
	Evidences []model.EvidenceSubmission `json:"evidences" binding:"required"`
}

func (c *Controller) submitShieldEvidences(ctx *gin.Context) {
	shieldID := ctx.Param("id")

	var req submitShieldEvidencesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	// Get the shield to find the asset ID
	shield, err := c.Svcs.DataSvc.RetrieveShieldByID(ctx, shieldID)
	if err != nil {
		if err == data.ErrNotFound {
			newError(ctx, http.StatusNotFound, err)
			return
		}
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := c.Svcs.FlowSvc.SubmitShieldEvidences(ctx, shield.AssetID, req.Evidences); err != nil {
		newError(ctx, submissionErrorStatus(err), err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "evidences submitted"})
}

type finalizeShieldEvidencesRequest struct {
	Evidences []model.EvidenceSubmission `json:"evidences"`
}

func (c *Controller) finalizeShieldEvidences(ctx *gin.Context) {
	shieldID := ctx.Param("id")

	var req finalizeShieldEvidencesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	// Get the shield to find the asset ID
	shield, err := c.Svcs.DataSvc.RetrieveShieldByID(ctx, shieldID)
	if err != nil {
		if err == data.ErrNotFound {
			newError(ctx, http.StatusNotFound, err)
			return
		}
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	if err := c.Svcs.FlowSvc.FinalizeShieldEvidences(ctx, shield.AssetID, req.Evidences); err != nil {
		newError(ctx, submissionErrorStatus(err), err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "evidences finalized"})
}
