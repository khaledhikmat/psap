package minions

import (
	"context"
	"strings"
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
	Running               bool                               `json:"running"`
	InitialActivitiesDone bool                               `json:"initialActivitiesDone"`
	FinalActivitiesDone   bool                               `json:"finalActivitiesDone"`
	SignalsCount          int                                `json:"signalsCount"`
	TimerInvocationsCount int                                `json:"timerInvocationsCount"`
}

type MinionRequest struct {
	ID        string
	ManagerID string
	Kind      string
	Command   string
	Args      []any
}

func (r MinionRequest) Marshall() string {
	return r.ID + ":" + r.ManagerID + ":" + r.Kind + ":" + r.Command
}

func (r MinionRequest) Unmarshall(json string) MinionRequest {
	parts := strings.Split(json, ":")
	return MinionRequest{
		ID:        parts[0],
		ManagerID: parts[1],
		Kind:      parts[2],
		Command:   parts[3],
	}
}

type IMinion interface {
	Start(canx context.Context) error
	State() MinionState
	Signal(payload MinionRequest) error
	Stop()
}
