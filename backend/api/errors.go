package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab.rndalpha.com/grp-esg-soi-threat-detection/stream-processor/model"
)

func (c *Controller) listErrors(ctx *gin.Context) {
	sourceType := ctx.Request.URL.Query().Get("source")
	if sourceType == "" {
		sourceType = "main"
	}

	timeRange := ctx.Request.URL.Query().Get("timeRange")
	if timeRange == "" {
		timeRange = "1h"
	}

	result, err := c.Svcs.DataSvc.RetrieveErrors(ctx, sourceType, timeRange)
	if err != nil {
		newError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(result) == 0 {
		result = []model.CustomEvent{
			{
				Source:      "Agent",
				Severity:    "High",
				Inner:       "Big Exception",
				Message:     "Unable to reach inference layer",
				StackTrace:  "at ....",
				SourceAttrs: map[string]interface{}{"version": "1.10", "region": "SAT", "building": "B"},
			},
			{
				Source:      "Agent",
				Severity:    "Low",
				Inner:       "Small Exception",
				Message:     "Unable to submit layer",
				StackTrace:  "at ....",
				SourceAttrs: map[string]interface{}{"version": "1.10", "region": "SAT", "building": "B"},
			},
		}
	}

	ctx.JSON(http.StatusOK, result)
}
