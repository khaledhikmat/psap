package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/answers"
	"github.com/khaledhikmat/psap/service/data"
)

var (
	errIncompleteShield = errors.New("shield is incomplete")
)

func newRecordID(prefix string) (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate record ID: %w", err)
	}
	return prefix + hex.EncodeToString(bytes[:]), nil
}

func writeDataError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, data.ErrNotFound):
		newError(ctx, http.StatusNotFound, err)
	case errors.Is(err, data.ErrAlreadyExists):
		newError(ctx, http.StatusConflict, err)
	case errors.Is(err, data.ErrInvalidState):
		newError(ctx, http.StatusConflict, err)
	default:
		newError(ctx, http.StatusInternalServerError, err)
	}
}

func writeSubmissionError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, answers.ErrInvalid), errors.Is(err, errIncompleteShield):
		newError(ctx, http.StatusBadRequest, err)
	case errors.Is(err, data.ErrInvalidState):
		newError(ctx, http.StatusConflict, err)
	default:
		newError(ctx, http.StatusInternalServerError, err)
	}
}

func (c *Controller) createShieldForAsset(ctx *gin.Context, asset model.Asset) error {
	if asset.BusinessOwnerID == nil || asset.BusinessManagerID == nil {
		return errors.New("asset needs a business owner and manager before it can be shielded")
	}
	shieldID, err := newRecordID("shield-")
	if err != nil {
		return err
	}
	shield := model.Shield{
		ID:        shieldID,
		AssetID:   asset.ID,
		State:     model.WaitingForAnswersShieldState,
		CreatedBy: "shield-api",
		UpdatedBy: "shield-api",
	}
	return c.DataSvc.InitiateShield(ctx, shield)
}
