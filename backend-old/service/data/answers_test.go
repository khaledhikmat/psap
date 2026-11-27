package data

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/answers"
)

type fileConfig struct{ file string }

func (c fileConfig) SQLiteFile() string { return c.file }

func TestSaveLaunchAnswersMultipleSelectionsAndHistory(t *testing.T) {
	ctx := context.Background()
	svc := NewSQLLite(ctx, fileConfig{filepath.Join(t.TempDir(), "test.db")})
	if svc == nil {
		t.Fatal("unable to open database")
	}
	defer svc.Finalize()

	const launchID = "launch-1"
	const questionID = "launch-room-function-1"
	save := func(ids ...string) {
		t.Helper()
		sub := []model.AnswerSubmission{{QuestionID: questionID, PredefinedAnswerIDs: ids}}
		if err := svc.SaveLaunchAnswers(ctx, launchID, sub, "tester"); err != nil {
			t.Fatal(err)
		}
	}
	selected := func() []string {
		t.Helper()
		details, err := svc.RetrieveUserAnswersByLaunchID(ctx, launchID)
		if err != nil || len(details) != 1 {
			t.Fatalf("answers = %v, err = %v", details, err)
		}
		ids := []string{}
		for _, s := range details[0].Selections {
			ids = append(ids, s.PredefinedAnswerID)
		}
		return ids
	}

	yes, no := questionID+"-1", questionID+"-2"
	save(yes)
	save(yes) // repeating a submission changes nothing, including the history
	if got := selected(); len(got) != 1 || got[0] != yes {
		t.Fatalf("selected = %v, want [%s]", got, yes)
	}
	history, _ := svc.RetrieveUserAnswerHistoryByLaunchID(ctx, launchID)
	if len(history) != 1 || history[0].PreviousPredefinedAnswerID != nil || history[0].NewPredefinedAnswerID != yes {
		t.Fatalf("history = %+v", history)
	}

	save(no) // replaces yes with no
	if got := selected(); len(got) != 1 || got[0] != no {
		t.Fatalf("selected = %v, want [%s]", got, no)
	}
	history, _ = svc.RetrieveUserAnswerHistoryByLaunchID(ctx, launchID)
	if len(history) != 2 || history[0].PreviousPredefinedAnswerID == nil || *history[0].PreviousPredefinedAnswerID != yes || history[0].NewPredefinedAnswerID != no {
		t.Fatalf("history = %+v", history)
	}

	save(yes, no) // adds yes, keeps no
	if got := selected(); len(got) != 2 {
		t.Fatalf("selected = %v, want both", got)
	}
}

func TestValidateAnswers(t *testing.T) {
	ctx := context.Background()
	svc := NewSQLLite(ctx, fileConfig{filepath.Join(t.TempDir(), "test.db")})
	if svc == nil {
		t.Fatal("unable to open database")
	}
	defer svc.Finalize()

	questions, _ := svc.RetrieveQuestionsByCategoryType(ctx, model.LaunchQuestionCategory)
	predefined, _ := svc.RetrievePredefinedAnswersByCategoryType(ctx, model.LaunchQuestionCategory)
	multi, q1, q2 := questions[0], questions[1], questions[2] // room function allows several answers
	if !multi.AllowMultipleAnswers || q1.AllowMultipleAnswers || q2.AllowMultipleAnswers {
		t.Fatalf("unexpected seed: multiple = %v/%v/%v", multi.AllowMultipleAnswers, q1.AllowMultipleAnswers, q2.AllowMultipleAnswers)
	}
	a1, b1, a2 := q1.ID+"-1", q1.ID+"-2", q2.ID+"-1"

	for name, sub := range map[string][]model.AnswerSubmission{
		"unknown question":         {{QuestionID: "nope", PredefinedAnswerIDs: []string{a1}}},
		"no selection":             {{QuestionID: q1.ID}},
		"answer of other question": {{QuestionID: q1.ID, PredefinedAnswerIDs: []string{a2}}},
		"several for single":       {{QuestionID: q1.ID, PredefinedAnswerIDs: []string{a1, b1}}},
		"question twice":           {{QuestionID: q1.ID, PredefinedAnswerIDs: []string{a1}}, {QuestionID: q1.ID, PredefinedAnswerIDs: []string{b1}}},
	} {
		if err := answers.Validate(questions, predefined, sub); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	ok := []model.AnswerSubmission{
		{QuestionID: q1.ID, PredefinedAnswerIDs: []string{a1}},
		{QuestionID: q2.ID, PredefinedAnswerIDs: []string{a2}},
		{QuestionID: multi.ID, PredefinedAnswerIDs: []string{multi.ID + "-1", multi.ID + "-3"}},
	}
	if err := answers.Validate(questions, predefined, ok); err != nil {
		t.Errorf("valid submission rejected: %v", err)
	}
}

func TestSeedAddsAdminToExistingDatabase(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "test.db")
	svc := NewSQLLite(ctx, fileConfig{file})
	if svc == nil {
		t.Fatal("unable to open database")
	}
	svc.Finalize()

	count := func() int {
		s := NewSQLLite(ctx, fileConfig{file})
		defer s.Finalize()
		workers, err := s.RetrieveWorkers(ctx)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, w := range workers {
			if strings.HasPrefix(w.ID, "worker-admin") {
				n++
			}
		}
		return n
	}
	if got := count(); got != 1 {
		t.Fatalf("admins = %d, want 1 (reopening must not duplicate)", got)
	}
}
