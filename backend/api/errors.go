package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/service/flow"
)

func (c *Controller) listErrors(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, []interface{}{})
}

// submissionErrorStatus maps an error from submitting answers or evidences: invalid or incomplete input is
// the caller's mistake, anything else is ours.
func submissionErrorStatus(err error) int {
	if errors.Is(err, flow.ErrInvalidAnswer) || errors.Is(err, flow.ErrIncompleteShield) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
