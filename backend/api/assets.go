package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/data"
)

type createAssetRequest struct {
	Name              string `json:"name" binding:"required"`
	Campus            string `json:"campus" binding:"required"`
	Building          string `json:"building" binding:"required"`
	Section           string `json:"section"`
	Level             string `json:"level"`
	Wing              string `json:"wing"`
	Floor             string `json:"floor"`
	Space             string `json:"space"`
	Priority          string `json:"priority"`
	BusinessOwnerID   string `json:"businessOwnerId" binding:"required"`
	BusinessManagerID string `json:"businessManagerId" binding:"required"`
}

// This returns all assets and let the UI filter
func (c *Controller) listAssets(ctx *gin.Context) {
	result, err := c.DataSvc.RetrieveAssets(ctx)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) createAsset(ctx *gin.Context) {
	var req createAssetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		newError(ctx, http.StatusBadRequest, err)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Campus = strings.TrimSpace(req.Campus)
	req.Building = strings.TrimSpace(req.Building)
	req.BusinessOwnerID = strings.TrimSpace(req.BusinessOwnerID)
	req.BusinessManagerID = strings.TrimSpace(req.BusinessManagerID)
	if req.Name == "" || req.Campus == "" || req.Building == "" {
		newError(ctx, http.StatusBadRequest, errors.New("name, campus, and building are required"))
		return
	}

	for _, workerID := range []string{req.BusinessOwnerID, req.BusinessManagerID} {
		if _, err := c.DataSvc.RetrieveWorkerByID(ctx, workerID); err != nil {
			if errors.Is(err, data.ErrNotFound) {
				newError(ctx, http.StatusBadRequest, errors.New("business owner and manager must be known workers"))
				return
			}
			newError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "normal"
	}
	asset := model.Asset{
		ID:                "asset-" + hex.EncodeToString(idBytes),
		Name:              req.Name,
		Campus:            req.Campus,
		Building:          req.Building,
		Section:           strings.TrimSpace(req.Section),
		Level:             strings.TrimSpace(req.Level),
		Wing:              strings.TrimSpace(req.Wing),
		Floor:             strings.TrimSpace(req.Floor),
		Space:             strings.TrimSpace(req.Space),
		Priority:          priority,
		State:             model.PendingLaunchAssetState,
		BusinessOwnerID:   &req.BusinessOwnerID,
		BusinessManagerID: &req.BusinessManagerID,
		CreatedBy:         req.BusinessOwnerID,
		CreatedAt:         now,
		UpdatedBy:         req.BusinessOwnerID,
		UpdatedAt:         now,
	}
	if err := c.DataSvc.NewAsset(ctx, asset); err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusCreated, asset)
}

func (c *Controller) showAsset(ctx *gin.Context) {
	id := ctx.Param("id")

	result, err := c.DataSvc.RetrieveAssetByID(ctx, id)
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

func (c *Controller) listAssetLaunches(ctx *gin.Context) {
	assetID := ctx.Param("id")

	result, err := c.DataSvc.RetrieveLaunchesByAssetID(ctx, assetID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listAssetShields(ctx *gin.Context) {
	assetID := ctx.Param("id")

	result, err := c.DataSvc.RetrieveShieldsByAssetID(ctx, assetID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) listAssetBaselines(ctx *gin.Context) {
	assetID := ctx.Param("id")

	result, err := c.DataSvc.RetrieveBaselinesByAssetID(ctx, assetID)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}
