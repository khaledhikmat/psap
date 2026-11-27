package service

import (
	"context"

	"github.com/khaledhikmat/psap/service/config"
	"github.com/khaledhikmat/psap/service/data"
	"github.com/khaledhikmat/psap/service/flow"
)

type ServicesFactory struct {
	CanxCtx context.Context
	CfgSvc  config.IService
	DataSvc data.IService
	FlowSvc flow.IService
}
