// Package answers holds the rules shared by launches and shields for validating submitted
// answers and scoring them.
package answers

import (
	"errors"
	"fmt"

	"github.com/khaledhikmat/psap/model"
)

// ErrInvalid is returned (wrapped) for a submission that can never be saved. Retrying does not help.
var ErrInvalid = errors.New("invalid answer")

// Validate checks that every submission targets a known question, selects at least one of
// that question's predefined answers, has no duplicates, and selects several answers only
// when the question allows it.
func Validate(questions []model.Question, predefined []model.PredefinedAnswer, submissions []model.AnswerSubmission) error {
	byQuestion := make(map[string]model.Question, len(questions))
	for _, q := range questions {
		byQuestion[q.ID] = q
	}
	owner := make(map[string]string, len(predefined)) // predefined answer ID -> question ID
	for _, p := range predefined {
		owner[p.ID] = p.QuestionID
	}

	seen := make(map[string]bool, len(submissions))
	for _, s := range submissions {
		q, ok := byQuestion[s.QuestionID]
		if !ok {
			return fmt.Errorf("%w: %q is not a question of this assessment", ErrInvalid, s.QuestionID)
		}
		if seen[s.QuestionID] {
			return fmt.Errorf("%w: question %s is answered more than once", ErrInvalid, s.QuestionID)
		}
		seen[s.QuestionID] = true

		if len(s.PredefinedAnswerIDs) == 0 {
			return fmt.Errorf("%w: question %s needs at least one selected answer", ErrInvalid, s.QuestionID)
		}
		if len(s.PredefinedAnswerIDs) > 1 && !q.AllowMultipleAnswers {
			return fmt.Errorf("%w: question %s allows a single answer", ErrInvalid, s.QuestionID)
		}
		picked := make(map[string]bool, len(s.PredefinedAnswerIDs))
		for _, id := range s.PredefinedAnswerIDs {
			if owner[id] != s.QuestionID {
				return fmt.Errorf("%w: %q is not an answer of question %s", ErrInvalid, id, s.QuestionID)
			}
			if picked[id] {
				return fmt.Errorf("%w: answer %q is selected twice for question %s", ErrInvalid, id, s.QuestionID)
			}
			picked[id] = true
		}
	}
	return nil
}

// Points is the sum of the points of the selected predefined answers.
func Points(answers []model.UserAnswerDetail, predefined []model.PredefinedAnswer) int {
	points := make(map[string]int, len(predefined))
	for _, p := range predefined {
		points[p.ID] = p.Points
	}
	total := 0
	for _, a := range answers {
		for _, s := range a.Selections {
			total += points[s.PredefinedAnswerID]
		}
	}
	return total
}

// Answered counts the answers that have at least one selection.
func Answered(answers []model.UserAnswerDetail) int {
	n := 0
	for _, a := range answers {
		if len(a.Selections) > 0 {
			n++
		}
	}
	return n
}
