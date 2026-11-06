package minions

import (
	"context"
	"time"
)

// If true, the timer should be re-scheduled after the specified duration.
type MinionTimerProcessor func(ctx context.Context, t time.Time) (bool, time.Duration)

type MinionState struct {
	ID                    string                             `json:"id"`
	ManagerID             string                             `json:"managerId"`
	Kind                  string                             `json:"kind"`
	StartTime             *time.Time                         `json:"startTime"`
	EndTime               *time.Time                         `json:"endTime"`
	Timers                map[time.Time]MinionTimerProcessor `json:"timers"`
	InitialActivitiesDone bool                               `json:"initialActivitiesDone"`
	FinalActivitiesDone   bool                               `json:"finalActivitiesDone"`
	CommandsCount         int                                `json:"commandsCount"`
	TimerInvocationsCount int                                `json:"timerInvocationsCount"`
}

type MinionRequest struct {
	Command string
	Args    []any
}

type MinionResponse struct {
	ID        string
	ManagerID string
	Kind      string
	Command   string
	Result    map[string]any
}

type IMinion interface {
	Start(canx context.Context) error
	State() MinionState
	Query(req MinionRequest) (MinionResponse, error)
	Signal(payload MinionRequest) error
	Close()
}
