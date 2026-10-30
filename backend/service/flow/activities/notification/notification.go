package notification

import (
	"github.com/khaledhikmat/psap/service/config"
	"github.com/khaledhikmat/psap/service/data"
)

// Activities holds the dependencies shared by the notification activities.
type Activities struct {
	CfgSvc  config.IService
	DataSvc data.IService
}

func NewActivities(cfgSvc config.IService, dataSvc data.IService) *Activities {
	return &Activities{CfgSvc: cfgSvc, DataSvc: dataSvc}
}

func (a *Activities) Notify(req NotificationRequest) (NotificationResult, error) {
	// Implement the logic for creating a launch here
	// This is a placeholder implementation
	return NotificationResult{
		Success: true,
	}, nil
}
