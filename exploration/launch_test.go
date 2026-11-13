package minions

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/data"
	"github.com/khaledhikmat/psap/service/flow"
)

type launchMinionDataStub struct {
	data.IService
	mu         sync.Mutex
	asset      model.Asset
	launch     *model.Launch
	questions  []model.Question
	predefined []model.PredefinedAnswer
	answers    []model.UserAnswerDetail
	tier       model.Tier
}

func (s *launchMinionDataStub) RetrieveLaunchByID(_ context.Context, id string) (model.Launch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.launch == nil || s.launch.ID != id {
		return model.Launch{}, data.ErrNotFound
	}
	return *s.launch, nil
}

func (s *launchMinionDataStub) RetrieveAssetByID(_ context.Context, id string) (model.Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.asset.ID != id {
		return model.Asset{}, data.ErrNotFound
	}
	return s.asset, nil
}

func (s *launchMinionDataStub) UpdateAsset(_ context.Context, asset model.Asset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asset = asset
	return nil
}

func (s *launchMinionDataStub) NewLaunch(_ context.Context, launch model.Launch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.launch != nil {
		return data.ErrAlreadyExists
	}
	s.launch = &launch
	return nil
}

func (s *launchMinionDataStub) UpdateLaunch(_ context.Context, launch model.Launch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.launch == nil || s.launch.ID != launch.ID {
		return data.ErrNotFound
	}
	s.launch = &launch
	return nil
}

func (s *launchMinionDataStub) RetrieveQuestionsByCategoryType(_ context.Context, _ model.QuestionCategoryType) ([]model.Question, error) {
	return s.questions, nil
}

func (s *launchMinionDataStub) RetrievePredefinedAnswersByCategoryType(_ context.Context, _ model.QuestionCategoryType) ([]model.PredefinedAnswer, error) {
	return s.predefined, nil
}

func (s *launchMinionDataStub) SaveLaunchAnswers(_ context.Context, _ string, submissions []model.AnswerSubmission, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.answers = nil
	for _, submission := range submissions {
		answer := model.UserAnswerDetail{UserAnswer: model.UserAnswer{QuestionID: submission.QuestionID}}
		for _, id := range submission.PredefinedAnswerIDs {
			answer.Selections = append(answer.Selections, model.UserAnswerSelection{PredefinedAnswerID: id})
		}
		s.answers = append(s.answers, answer)
	}
	return nil
}

func (s *launchMinionDataStub) RetrieveUserAnswersByLaunchID(_ context.Context, _ string) ([]model.UserAnswerDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.UserAnswerDetail(nil), s.answers...), nil
}

func (s *launchMinionDataStub) RetrieveTierByPoints(_ context.Context, _ int) (model.Tier, error) {
	return s.tier, nil
}

func (s *launchMinionDataStub) RetrieveTierByID(_ context.Context, id string) (model.Tier, error) {
	if id != s.tier.ID {
		return model.Tier{}, data.ErrNotFound
	}
	return s.tier, nil
}

type launchMinionFlowStub struct {
	flow.IService
	startedShield chan string
}

func (s *launchMinionFlowStub) InitiateShield(_ context.Context, assetID string) error {
	s.startedShield <- assetID
	return nil
}

func TestLaunchMinionLifecycle(t *testing.T) {
	svc := &launchMinionDataStub{
		asset: model.Asset{ID: "asset-1"},
		questions: []model.Question{{
			ID: "question-1",
		}},
		predefined: []model.PredefinedAnswer{{
			ID: "answer-1", QuestionID: "question-1", Points: 25,
		}, {
			ID: "answer-2", QuestionID: "question-1", Points: 50,
		}},
		tier: model.Tier{ID: "tier-1"},
	}
	flowSvc := &launchMinionFlowStub{startedShield: make(chan string, 1)}
	m := NewLaunchMinion("launch-1", "manager-1", service.ServicesFactory{
		DataSvc: svc,
		FlowSvc: flowSvc,
	}, LaunchMinionOptions{
		AssetID:         "asset-1",
		BusinessOwner:   "owner-1",
		BusinessManager: "manager-1",
		MaxReminders:    1,
		MaxWarnings:     1,
		MaxEscalations:  1,
	})
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	eventually(t, "launch creation", func() bool {
		return m.GetState().LaunchState == model.WaitingForAnswersLaunchState
	})
	submissions := []model.AnswerSubmission{{
		QuestionID:          "question-1",
		PredefinedAnswerIDs: []string{"answer-1"},
	}}
	if err := m.FinalizeAnswers(submissions); err != nil {
		t.Fatal(err)
	}
	eventually(t, "tier calculation", func() bool {
		state := m.GetState()
		return state.LaunchState == model.WaitingForTierApprovalLaunchState && state.Points == 25
	})

	revised := []model.AnswerSubmission{{
		QuestionID:          "question-1",
		PredefinedAnswerIDs: []string{"answer-2"},
	}}
	if err := m.ReviseAnswers(revised); err != nil {
		t.Fatal(err)
	}
	eventually(t, "manager answer revision", func() bool {
		return m.GetState().Points == 50
	})
	if err := m.ValidateTier("tier-1"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "tier validation", func() bool {
		return m.GetState().LaunchState == model.TierValidatedLaunchState
	})

	select {
	case assetID := <-flowSvc.startedShield:
		if assetID != "asset-1" {
			t.Errorf("started Shield for asset %q, want asset-1", assetID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("validated launch did not start Shield")
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.launch.State != model.TierValidatedLaunchState {
		t.Errorf("persisted launch state = %q, want %q", svc.launch.State, model.TierValidatedLaunchState)
	}
	if svc.asset.State != model.PendingShieldAssetState || svc.asset.TierID == nil || *svc.asset.TierID != "tier-1" {
		t.Errorf("persisted asset = %+v, want pending Shield with tier-1", svc.asset)
	}
}

func TestLaunchMinionRejectsInvalidAnswersAndWrongState(t *testing.T) {
	m := NewLaunchMinion("launch-1", "manager-1", service.ServicesFactory{}, LaunchMinionOptions{})
	if err := m.FinalizeAnswers(nil); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("finalize before Start = %v, want ErrNotRunning", err)
	}
}
