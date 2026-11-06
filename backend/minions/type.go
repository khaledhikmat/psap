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
	InitialActivitiesDone bool                               `json:"initialActivitiesDone"`
	FinalActivitiesDone   bool                               `json:"finalActivitiesDone"`
	CommandsCount         int                                `json:"commandsCount"`
	TimerInvocationsCount int                                `json:"timerInvocationsCount"`
}

type MinionRequest struct {
	ID         string
	BusinessID string
	ManagerID  string
	Kind       string
	Command    string
	Args       []any
}

func (r MinionRequest) Marshall() string {
	return r.ID + ":" + r.BusinessID + ":" + r.ManagerID + ":" + r.Kind + ":" + r.Command
}

func (r MinionRequest) Unmarshall(json string) MinionRequest {
	parts := strings.Split(json, ":")
	return MinionRequest{
		ID:         parts[0],
		BusinessID: parts[1],
		ManagerID:  parts[2],
		Kind:       parts[3],
		Command:    parts[4],
	}
}

type IMinion interface {
	Start(canx context.Context) error
	State() MinionState
	Signal(payload MinionRequest) error
	Stop()
}
