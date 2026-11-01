package mode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/answers"
	"github.com/khaledhikmat/psap/service/data"
	"github.com/khaledhikmat/psap/service/flow"
	"github.com/khaledhikmat/psap/service/flow/helpers"
	"github.com/khaledhikmat/psap/service/flow/workflows"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

const cliUsage = `Launch and Shield workflow starter / test CLI. Run a flowworker in another terminal first.

Usage: MODE_PROC=cli go run . <command> [args]

Data:
  assets                                  list assets with their business owner and manager
  questions                               list launch questions with their answer choices and points
  shield-questions                        list shield questions with their answer choices and points
  tiers                                   list tiers and their point ranges
  answers  <assetID>                      list the selected answers of the asset's current launch
  shield-answers <assetID>                list the selected answers of the asset's current shield
  evidences <assetID>                     list the evidences of the asset's current shield
  answer-history <assetID>                list answer change history for the asset's current launch
  shield-answer-history <assetID>         list answer change history for the asset's current shield

Launch commands:
  start    <assetID>                      start a launch
  query    <assetID>                      print the launch's current state
  submit   <assetID> questionID=answerID[,answerID…]…   submit (partial) answers; several
                                          answerIDs only for questions that allow multiple answers
  finalize <assetID> [questionID=answerID…] save any last answers, lock them and calculate the tier
                                          (unanswered questions earn no points)
  validate <assetID> [tierID]             accept the calculated tier, or override it
  wait     <assetID>                      block until the launch completes, print result

Shield commands:
  shield-start <assetID>                  start a shield
  shield-query <assetID>                  print the shield's current state
  shield-submit-answers <assetID> questionID=answerID[,answerID…]…   submit (partial) shield answers
  shield-finalize-answers <assetID> [questionID=answerID…] finalize shield answers
  shield-submit-evidences <assetID> questionID=desc:url… submit (partial) evidences for answered questions
  shield-finalize-evidences <assetID> [questionID=desc:url…] finalize evidences
  shield-wait <assetID>                   block until the shield completes, print result

Scenarios (assetID optional, defaults to the first asset):
  happy     [assetID]  start → partial submit → finalize → validate, checking points, tier and
                       the database after each step, and that invalid steps are rejected
  relaunch  [assetID]  relaunching mid-launch cancels the running launch (answers kept) and
                       starts a fresh one; a tier override is applied; relaunching a
                       completed launch works too
  timeout   [assetID]  start with short timeouts and watch reminders, warnings and
                       escalations fire until the launch times out (~25s)
  shield-happy [assetID] start shield → submit answers → submit evidences → finalize both
`

// strVal safely dereferences a string pointer, returning empty string if nil
func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// CLI drives the Launch workflow from the command line. Actions go through the flow
// service, the same path the API uses; a raw Temporal client is used only to observe the
// workflow (full query state, waiting for the result) and to start with short timeouts.
func CLI(canxCtx context.Context, pod string, svcs service.ServicesFactory) error {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(cliUsage)
		return nil
	}

	if svcs.FlowSvc == nil || svcs.DataSvc == nil {
		return errors.New("flow and data services must be provided")
	}
	c, err := client.Dial(client.Options{})
	if err != nil {
		return fmt.Errorf("unable to create Temporal client: %w", err)
	}
	defer c.Close()

	cli := &launchCLI{ctx: canxCtx, flow: svcs.FlowSvc, data: svcs.DataSvc, client: c}
	cmd, rest := args[0], args[1:]
	err = cli.run(cmd, rest)
	if err != nil {
		fmt.Printf("✗ %s failed: %v\n", cmd, err)
		return err
	}
	return nil
}

type launchCLI struct {
	ctx    context.Context
	flow   flow.IService
	data   data.IService
	client client.Client
}

func (c *launchCLI) run(cmd string, args []string) error {
	switch cmd {
	case "assets":
		return c.listAssets()
	case "questions":
		return c.listQuestions()
	case "shield-questions":
		return c.listShieldQuestions()
	case "tiers":
		return c.listTiers()
	case "answers":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		return c.listAnswers(asset)
	case "shield-answers":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		return c.listShieldAnswers(asset)
	case "evidences":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		return c.listEvidences(asset)
	case "answer-history":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		return c.listAnswerHistory(asset)
	case "shield-answer-history":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		return c.listShieldAnswerHistory(asset)
	case "start":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		return c.start(asset)
	case "query":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		_, err = c.query(asset)
		return err
	case "submit", "finalize":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		answers, err := parseAnswers(args[1:])
		if err != nil {
			return err
		}
		if cmd == "submit" {
			if len(answers) == 0 {
				return errors.New("missing questionID=answerID answers")
			}
			return c.submit(asset, answers)
		}
		return c.finalize(asset, answers)
	case "validate":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		tier := ""
		if len(args) > 1 {
			tier = args[1]
		}
		return c.validate(asset, tier)
	case "wait":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		_, err = c.wait(asset)
		return err
	case "shield-start":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		return c.shieldStart(asset)
	case "shield-query":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		_, err = c.shieldQuery(asset)
		return err
	case "shield-submit-answers":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		answers, err := parseAnswers(args[1:])
		if err != nil {
			return err
		}
		if len(answers) == 0 {
			return errors.New("missing questionID=answerID answers")
		}
		return c.shieldSubmitAnswers(asset, answers)
	case "shield-finalize-answers":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		answers, err := parseAnswers(args[1:])
		if err != nil {
			return err
		}
		return c.shieldFinalizeAnswers(asset, answers)
	case "shield-submit-evidences":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		evidences, err := parseEvidences(args[1:])
		if err != nil {
			return err
		}
		if len(evidences) == 0 {
			return errors.New("missing questionID=description:url evidences")
		}
		return c.shieldSubmitEvidences(asset, evidences)
	case "shield-finalize-evidences":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		evidences, err := parseEvidences(args[1:])
		if err != nil {
			return err
		}
		return c.shieldFinalizeEvidences(asset, evidences)
	case "shield-wait":
		asset, err := argAt(args, 0, "assetID")
		if err != nil {
			return err
		}
		_, err = c.shieldWait(asset)
		return err
	case "happy", "relaunch", "timeout", "shield-happy":
		asset, err := c.assetOrFirst(args)
		if err != nil {
			return err
		}
		switch cmd {
		case "happy":
			return c.happyScenario(asset)
		case "relaunch":
			return c.relaunchScenario(asset)
		case "timeout":
			return c.timeoutScenario(asset)
		default:
			return c.shieldHappyScenario(asset)
		}
	default:
		fmt.Print(cliUsage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

// Scenarios

func (c *launchCLI) happyScenario(asset string) error {
	fmt.Printf("== happy path: asset %s\n", asset)
	questions, err := c.data.RetrieveQuestionsByCategoryType(c.ctx, model.LaunchQuestionCategory)
	if err != nil {
		return err
	}
	if len(questions) < 6 {
		return fmt.Errorf("need at least 6 launch questions, have %d", len(questions))
	}
	byQuestion, predefined, err := c.predefinedAnswers(model.LaunchQuestionCategory)
	if err != nil {
		return err
	}

	if err := c.start(asset); err != nil {
		return err
	}
	r, err := c.waitFor(asset, fmt.Sprintf("waiting for answers to %d questions", len(questions)), func(r workflows.LaunchResult) bool {
		return r.State == model.WaitingForAnswersLaunchState && r.Questions == len(questions)
	})
	if err != nil {
		return err
	}
	launchID := r.ID

	// Invalid answers are rejected before reaching the workflow.
	if err := expectErr(c.flow.SubmitLaunchAnswers(c.ctx, asset, []model.AnswerSubmission{{QuestionID: "no-such-question", PredefinedAnswerIDs: []string{"x"}}}),
		flow.ErrInvalidAnswer, "unknown question"); err != nil {
		return err
	}
	if err := expectErr(c.flow.SubmitLaunchAnswers(c.ctx, asset, []model.AnswerSubmission{{QuestionID: questions[0].ID, PredefinedAnswerIDs: []string{"no-such-answer"}}}),
		flow.ErrInvalidAnswer, "unknown answer"); err != nil {
		return err
	}
	if err := expectErr(c.flow.SubmitLaunchAnswers(c.ctx, asset, []model.AnswerSubmission{{QuestionID: questions[0].ID, PredefinedAnswerIDs: []string{byQuestion[questions[1].ID][0].ID}}}),
		flow.ErrInvalidAnswer, "answer of another question"); err != nil {
		return err
	}
	for _, q := range questions {
		if q.AllowMultipleAnswers {
			continue
		}
		both := []string{byQuestion[q.ID][0].ID, byQuestion[q.ID][1].ID}
		if err := expectErr(c.flow.SubmitLaunchAnswers(c.ctx, asset, []model.AnswerSubmission{{QuestionID: q.ID, PredefinedAnswerIDs: both}}),
			flow.ErrInvalidAnswer, "several answers to a single-answer question"); err != nil {
			return err
		}
		break
	}
	// A question that allows several answers accepts them, and a repeated submission is harmless.
	for _, q := range questions {
		if !q.AllowMultipleAnswers {
			continue
		}
		both := []string{byQuestion[q.ID][0].ID, byQuestion[q.ID][1].ID}
		if err := c.submit(asset, []model.AnswerSubmission{{QuestionID: q.ID, PredefinedAnswerIDs: both}}); err != nil {
			return err
		}
		if err := c.expectStoredAnswers(launchID, []model.AnswerSubmission{{QuestionID: q.ID, PredefinedAnswerIDs: both}}); err != nil {
			return err
		}
		break
	}
	if err := expectErr(c.flow.ValidateTier(c.ctx, asset, ""), flow.ErrInvalidLaunchState, "validate before finalize"); err != nil {
		return err
	}

	// Pick a different predefined answer for each question. Submit the first half now, the
	// rest (minus 2 left blank, which earn no points) when finalizing. This replaces the
	// two answers selected above for a multiple-answer question.
	all := pickAnswers(questions, byQuestion, func(i, n int) int { return i % n })
	half := len(all) / 2
	first, rest := all[:half], all[half:len(all)-2]

	if err := c.submit(asset, first); err != nil {
		return err
	}
	if _, err := c.waitFor(asset, fmt.Sprintf("%d answered, no tier yet", half), func(r workflows.LaunchResult) bool {
		return r.State == model.WaitingForAnswersLaunchState && r.Answered == half && r.TierID == ""
	}); err != nil {
		return err
	}
	if err := c.expectStoredAnswers(launchID, first); err != nil {
		return err
	}

	// Changing an answer is recorded in the history: at least one entry per first answer, plus the change.
	changed := model.AnswerSubmission{
		QuestionID:          first[0].QuestionID,
		PredefinedAnswerIDs: []string{byQuestion[first[0].QuestionID][1].ID},
	}
	if err := c.submit(asset, []model.AnswerSubmission{changed}); err != nil {
		return err
	}
	first[0] = changed
	if err := c.expectStoredAnswers(launchID, first); err != nil {
		return err
	}
	history, err := c.data.RetrieveUserAnswerHistoryByLaunchID(c.ctx, launchID)
	if err != nil {
		return err
	}
	if err := expect(len(history) >= half+1, "answer history has %d entries, want at least %d", len(history), half+1); err != nil {
		return err
	}
	fmt.Printf("  ✓ answer history: %d entries\n", len(history))

	submitted := append(append([]model.AnswerSubmission{}, first...), rest...)
	wantPoints := submissionPoints(submitted, predefined)
	wantTier, err := c.data.RetrieveTierByPoints(c.ctx, wantPoints)
	if err != nil {
		return err
	}
	if err := c.finalize(asset, rest); err != nil {
		return err
	}
	r, err = c.waitFor(asset, fmt.Sprintf("finalized: %d points → %s, waiting for tier approval", wantPoints, wantTier.ID),
		func(r workflows.LaunchResult) bool {
			return r.State == model.WaitingForTierApprovalLaunchState && r.Points == wantPoints && r.TierID == wantTier.ID
		})
	if err != nil {
		return err
	}
	if err := c.expectStoredAnswers(launchID, submitted); err != nil {
		return err
	}
	if err := c.waitForPersisted(launchID, model.WaitingForTierApprovalLaunchState, wantTier.ID); err != nil {
		return err
	}

	// Answers are locked once finalized.
	if err := expectErr(c.flow.SubmitLaunchAnswers(c.ctx, asset, first[:1]), flow.ErrInvalidLaunchState, "submit after finalize"); err != nil {
		return err
	}

	if err := c.validate(asset, ""); err != nil {
		return err
	}
	res, err := c.wait(asset)
	if err != nil {
		return err
	}
	if err := expect(res.State == model.TierValidatedLaunchState && res.TierID == wantTier.ID,
		"result state=%s tier=%s, want %s/%s", res.State, res.TierID, model.TierValidatedLaunchState, wantTier.ID); err != nil {
		return err
	}
	if err := c.expectAsset(asset, launchID, wantTier.ID); err != nil {
		return err
	}
	fmt.Println("✓ happy path passed")
	return nil
}

// relaunchScenario relaunches an asset mid-launch: the running launch must be cancelled
// (workflow stopped, launch marked cancelled, its answers kept) and a fresh one started.
func (c *launchCLI) relaunchScenario(asset string) error {
	fmt.Printf("== relaunch: asset %s\n", asset)
	questions, err := c.data.RetrieveQuestionsByCategoryType(c.ctx, model.LaunchQuestionCategory)
	if err != nil {
		return err
	}
	if err := c.start(asset); err != nil {
		return err
	}
	old, err := c.waitForState(asset, model.WaitingForAnswersLaunchState)
	if err != nil {
		return err
	}
	byQuestion, _, err := c.predefinedAnswers(model.LaunchQuestionCategory)
	if err != nil {
		return err
	}
	partial := pickAnswers(questions[:5], byQuestion, func(i, n int) int { return 0 })
	if err := c.submit(asset, partial); err != nil {
		return err
	}
	if _, err := c.waitFor(asset, "5 answered", func(r workflows.LaunchResult) bool { return r.Answered == 5 }); err != nil {
		return err
	}

	// Relaunch while the first launch is still waiting for answers.
	if err := c.start(asset); err != nil {
		return fmt.Errorf("relaunch: %w", err)
	}
	desc, err := c.client.DescribeWorkflowExecution(c.ctx, workflows.LaunchWorkflowID(asset), old.ID)
	if err != nil {
		return err
	}
	if err := expect(desc.GetWorkflowExecutionInfo().GetStatus() == enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED,
		"old workflow status %s, want CANCELED", desc.GetWorkflowExecutionInfo().GetStatus()); err != nil {
		return err
	}
	fmt.Printf("  ✓ old workflow run %s cancelled\n", old.ID)
	if err := c.waitForPersisted(old.ID, model.CancelledLaunchState, ""); err != nil {
		return err
	}
	if err := c.expectStoredAnswers(old.ID, partial); err != nil {
		return err
	}

	r, err := c.waitFor(asset, "new launch waiting for answers, none answered", func(r workflows.LaunchResult) bool {
		return r.State == model.WaitingForAnswersLaunchState && r.ID != old.ID && r.Answered == 0 && r.Questions == len(questions)
	})
	if err != nil {
		return err
	}
	if err := c.expectStoredAnswers(r.ID, nil); err != nil {
		return err
	}

	// Finish the new launch with a tier override, then relaunch the completed asset.
	if err := c.finalizeAndValidate(asset, "tier-2"); err != nil {
		return err
	}
	res, err := c.wait(asset)
	if err != nil {
		return err
	}
	if err := expect(res.ID == r.ID && res.TierID == "tier-2", "result id=%s tier=%s, want %s/tier-2", res.ID, res.TierID, r.ID); err != nil {
		return err
	}
	if err := c.expectAsset(asset, res.ID, "tier-2"); err != nil {
		return err
	}

	if err := c.start(asset); err != nil {
		return fmt.Errorf("relaunch after completion: %w", err)
	}
	fmt.Println("→ relaunch after completion accepted")
	if err := expectErr(c.flow.ValidateTier(c.ctx, asset, "tier-1"), flow.ErrInvalidLaunchState, "validate before finalize"); err != nil {
		return err
	}
	if err := c.finalizeAndValidate(asset, ""); err != nil {
		return err
	}
	res, err = c.wait(asset)
	if err != nil {
		return err
	}
	if err := c.expectAsset(asset, res.ID, res.TierID); err != nil {
		return err
	}
	fmt.Println("✓ relaunch scenario passed")
	return nil
}

// finalizeAndValidate finalizes with no further answers, then validates with tierID.
func (c *launchCLI) finalizeAndValidate(asset, tierID string) error {
	if err := c.finalize(asset, nil); err != nil {
		return err
	}
	if _, err := c.waitForState(asset, model.WaitingForTierApprovalLaunchState); err != nil {
		return err
	}
	if tierID != "" {
		if err := expectErr(c.flow.ValidateTier(c.ctx, asset, "no-such-tier"), flow.ErrInvalidTier, "unknown tier"); err != nil {
			return err
		}
	}
	return c.validate(asset, tierID)
}

// timeoutScenario starts the workflow directly, rather than through the flow service, so it
// can use timeouts short enough to watch. Expected timeline (seconds):
// reminders at 2 and 4, a warning at 5, an escalation at 12, timed out at 24.
func (c *launchCLI) timeoutScenario(asset string) error {
	fmt.Printf("== timeouts: asset %s\n", asset)
	a, err := c.data.RetrieveAssetByID(c.ctx, asset)
	if err != nil {
		return err
	}
	req := workflows.LaunchRequest{
		AssetID:           asset,
		BusinessOwner:     *a.BusinessOwnerID,
		BusinessManager:   *a.BusinessManagerID,
		ReminderTimeout:   2,
		MaxReminders:      2,
		WarningTimeout:    5,
		MaxWarnings:       1,
		EscalationTimeout: 12,
		MaxEscalations:    1,
	}
	_, err = c.client.ExecuteWorkflow(c.ctx, client.StartWorkflowOptions{
		ID:                                       workflows.LaunchWorkflowID(asset),
		TaskQueue:                                helpers.WorkerQueueName,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}, workflows.Launch, req)
	if err != nil {
		return err
	}
	fmt.Printf("→ started with reminders %ds×%d, warnings %ds×%d, escalations %ds×%d\n",
		req.ReminderTimeout, req.MaxReminders, req.WarningTimeout, req.MaxWarnings, req.EscalationTimeout, req.MaxEscalations)

	// Print each change in the nudge counters until the workflow completes.
	done := make(chan struct{})
	go func() {
		defer close(done)
		last := ""
		start := time.Now()
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
			r, err := c.queryState(asset)
			if err != nil {
				return // workflow completed (or is gone); wait() reports the outcome
			}
			now := fmt.Sprintf("reminders=%d warnings=%d escalations=%d state=%s", r.Reminders, r.Warnings, r.Escalations, r.State)
			if now != last {
				fmt.Printf("  t+%4.1fs  %s\n", time.Since(start).Seconds(), now)
				last = now
			}
			if r.State == model.TimedOutLaunchState {
				return
			}
		}
	}()

	<-done
	res, err := c.wait(asset)
	if err != nil {
		return err
	}
	if err := expect(res.State == model.TimedOutLaunchState && res.Reminders == 2 && res.Warnings == 1 && res.Escalations == 1,
		"got state=%s reminders=%d warnings=%d escalations=%d, want %s 2/1/1",
		res.State, res.Reminders, res.Warnings, res.Escalations, model.TimedOutLaunchState); err != nil {
		return err
	}
	fmt.Println("✓ timeout scenario passed")
	return nil
}

// shieldHappyScenario tests the Shield workflow end-to-end by submitting answers and evidences.
func (c *launchCLI) shieldHappyScenario(asset string) error {
	fmt.Printf("== shield happy path: asset %s\n", asset)
	questions, err := c.data.RetrieveQuestionsByCategoryType(c.ctx, model.ShieldQuestionCategory)
	if err != nil {
		return err
	}
	if len(questions) < 3 {
		return fmt.Errorf("need at least 3 shield questions, have %d", len(questions))
	}
	byQuestion, predefined, err := c.predefinedAnswers(model.ShieldQuestionCategory)
	if err != nil {
		return err
	}

	if err := c.shieldStart(asset); err != nil {
		return err
	}
	r, err := c.waitForShield(asset, fmt.Sprintf("waiting for answers to %d questions", len(questions)), func(r workflows.ShieldResult) bool {
		return r.State == model.WaitingForAnswersShieldState && r.Questions == len(questions)
	})
	if err != nil {
		return err
	}
	shieldID := r.ID

	if err := expectErr(c.flow.SubmitShieldAnswers(c.ctx, asset, []model.AnswerSubmission{{QuestionID: "no-such-question", PredefinedAnswerIDs: []string{"x"}}}),
		flow.ErrInvalidAnswer, "unknown question"); err != nil {
		return err
	}

	// Answer every question: the first half now, the rest when finalizing.
	half := len(questions) / 2
	all := pickAnswers(questions, byQuestion, func(i, n int) int { return (i * 2) % n })
	if err := c.shieldSubmitAnswers(asset, all[:half]); err != nil {
		return err
	}
	if _, err := c.waitForShield(asset, fmt.Sprintf("%d answered", half), func(r workflows.ShieldResult) bool {
		return r.Answered == half && r.State == model.WaitingForAnswersShieldState
	}); err != nil {
		return err
	}

	// Evidence can only be attached to an answered question, and answers can only be finalized
	// once every question is answered.
	if err := expectErr(c.flow.SubmitShieldEvidences(c.ctx, asset, []model.EvidenceSubmission{{QuestionID: questions[half].ID, Description: "x"}}),
		flow.ErrInvalidAnswer, "evidence for an unanswered question"); err != nil {
		return err
	}
	if err := expectErr(c.flow.FinalizeShieldAnswers(c.ctx, asset, all[half:len(all)-1]),
		flow.ErrIncompleteShield, "finalize with a question unanswered"); err != nil {
		return err
	}
	if err := c.shieldFinalizeAnswers(asset, all[half:]); err != nil {
		return err
	}
	if _, err := c.waitForShield(asset, "answers finalized, waiting for evidences", func(r workflows.ShieldResult) bool {
		return r.State == model.WaitingForEvidencesShieldState && r.Answered == len(questions)
	}); err != nil {
		return err
	}

	// Submit partial evidences (first question), then try to finalize without the rest.
	evidence := func(i int) model.EvidenceSubmission {
		return model.EvidenceSubmission{
			QuestionID:  questions[i].ID,
			Description: fmt.Sprintf("Test evidence %d", i+1),
			URL:         fmt.Sprintf("https://example.com/evidence%d", i+1),
		}
	}
	if err := c.shieldSubmitEvidences(asset, []model.EvidenceSubmission{evidence(0)}); err != nil {
		return err
	}
	if _, err := c.waitForShield(asset, "1 evidence submitted", func(r workflows.ShieldResult) bool {
		return r.EvidencesCount == 1 && r.State == model.WaitingForEvidencesShieldState
	}); err != nil {
		return err
	}
	remainingEvidences := []model.EvidenceSubmission{}
	for i := 1; i < len(questions); i++ {
		remainingEvidences = append(remainingEvidences, evidence(i))
	}
	if err := expectErr(c.flow.FinalizeShieldEvidences(c.ctx, asset, remainingEvidences[1:]),
		flow.ErrIncompleteShield, "finalize with evidence missing"); err != nil {
		return err
	}
	if err := c.shieldFinalizeEvidences(asset, remainingEvidences); err != nil {
		return err
	}

	// Wait for completion and check score
	res, err := c.shieldWait(asset)
	if err != nil {
		return err
	}
	if res.State != model.CompletedShieldState {
		return fmt.Errorf("expected state %s, got %s", model.CompletedShieldState, res.State)
	}
	wantScore := submissionPoints(all, predefined)
	if err := expect(res.Score == wantScore, "shield score %d, want %d", res.Score, wantScore); err != nil {
		return err
	}
	fmt.Printf("  ✓ shield completed with score=%d\n", res.Score)
	stored, err := c.data.RetrieveEvidencesByShieldID(c.ctx, shieldID)
	if err != nil {
		return err
	}
	if err := expect(len(stored) == len(questions), "stored %d evidences, want %d", len(stored), len(questions)); err != nil {
		return err
	}
	fmt.Printf("  ✓ stored %d evidences\n", len(stored))

	// Verify asset is now shielded
	a, err := c.data.RetrieveAssetByID(c.ctx, asset)
	if err != nil {
		return err
	}
	if a.State != model.ShieldedAssetState {
		return fmt.Errorf("expected asset state %s, got %s", model.ShieldedAssetState, a.State)
	}
	if strVal(a.LastShieldID) != shieldID {
		return fmt.Errorf("expected LastShieldID=%s, got %s", shieldID, strVal(a.LastShieldID))
	}
	fmt.Printf("  ✓ asset %s: state=%s lastShield=%s\n", asset, a.State, strVal(a.LastShieldID))

	fmt.Println("✓ shield happy path passed")
	return nil
}

// Data commands

func (c *launchCLI) listAssets() error {
	assets, err := c.data.RetrieveAssets(c.ctx)
	if err != nil {
		return err
	}
	for _, a := range assets {
		fmt.Printf("%-14s %-16s state=%-16s tier=%-7s owner=%s manager=%s lastLaunch=%s\n",
			a.ID, a.Name, a.State, strVal(a.TierID), c.workerName(strVal(a.BusinessOwnerID)), c.workerName(strVal(a.BusinessManagerID)), strVal(a.LastLaunchID))
	}
	return nil
}

func (c *launchCLI) listQuestions() error {
	return c.printQuestions(model.LaunchQuestionCategory)
}

// printQuestions lists the categories of the given type with their questions and answer choices.
func (c *launchCLI) printQuestions(t model.QuestionCategoryType) error {
	categories, err := c.data.RetrieveQuestionCategoriesByType(c.ctx, t)
	if err != nil {
		return err
	}
	total := 0
	for _, cat := range categories {
		fmt.Printf("%s — %s\n", cat.ID, cat.Title)
		questions, err := c.data.RetrieveQuestionsByCategoryID(c.ctx, cat.ID)
		if err != nil {
			return err
		}
		for _, q := range questions {
			flags := ""
			if q.AllowMultipleAnswers {
				flags += " [multiple]"
			}
			if q.EvidenceRequired {
				flags += " [evidence]"
			}
			fmt.Printf("  %s%s  %s\n", q.ID, flags, q.Text)
			choices, err := c.data.RetrievePredefinedAnswersByQuestionID(c.ctx, q.ID)
			if err != nil {
				return err
			}
			best := 0
			for _, a := range choices {
				fmt.Printf("      %-30s %3d pts  %s\n", a.ID, a.Points, a.Text)
				if q.AllowMultipleAnswers {
					best += max(a.Points, 0)
				} else {
					best = max(best, a.Points)
				}
			}
			total += best
		}
	}
	fmt.Printf("max points: %d\n", total)
	return nil
}

func (c *launchCLI) listTiers() error {
	tiers, err := c.data.RetrieveTiers(c.ctx)
	if err != nil {
		return err
	}
	for _, t := range tiers {
		fmt.Printf("%-7s %-7s %-9s points %3d–%-3d  %s\n", t.ID, t.Name, t.Title, t.FromPoint, t.ToPoint, t.Cadence)
	}
	return nil
}

func (c *launchCLI) listAnswers(asset string) error {
	a, err := c.data.RetrieveAssetByID(c.ctx, asset)
	if err != nil {
		return err
	}
	if a.LastLaunchID == nil || *a.LastLaunchID == "" {
		return fmt.Errorf("asset %s has not been launched", asset)
	}
	answers, err := c.data.RetrieveUserAnswersByLaunchID(c.ctx, *a.LastLaunchID)
	if err != nil {
		return err
	}
	printUserAnswers(answers)
	return nil
}

// printUserAnswers prints each answered question with the predefined answers selected for it.
func printUserAnswers(answers []model.UserAnswerDetail) {
	if len(answers) == 0 {
		fmt.Println("  no answers yet")
	}
	for _, ans := range answers {
		ids := make([]string, len(ans.Selections))
		for i, sel := range ans.Selections {
			ids[i] = sel.PredefinedAnswerID
		}
		fmt.Printf("  %-26s %s\n", ans.QuestionID, strings.Join(ids, ", "))
	}
}

func (c *launchCLI) listShieldQuestions() error {
	return c.printQuestions(model.ShieldQuestionCategory)
}

func (c *launchCLI) listShieldAnswers(asset string) error {
	a, err := c.data.RetrieveAssetByID(c.ctx, asset)
	if err != nil {
		return err
	}
	if a.LastShieldID == nil || *a.LastShieldID == "" {
		return fmt.Errorf("asset %s has no shield", asset)
	}
	answers, err := c.data.RetrieveUserAnswersByShieldID(c.ctx, *a.LastShieldID)
	if err != nil {
		return err
	}
	printUserAnswers(answers)
	return nil
}

func (c *launchCLI) listEvidences(asset string) error {
	a, err := c.data.RetrieveAssetByID(c.ctx, asset)
	if err != nil {
		return err
	}
	if a.LastShieldID == nil || *a.LastShieldID == "" {
		return fmt.Errorf("asset %s has no shield", asset)
	}
	evidences, err := c.data.RetrieveEvidencesByShieldID(c.ctx, *a.LastShieldID)
	if err != nil {
		return err
	}
	for _, evid := range evidences {
		desc := evid.Description
		if desc == "" {
			desc = "-"
		}
		url := evid.URL
		if url == "" {
			url = "-"
		}
		fmt.Printf("  %-26s desc=%-30s url=%s\n", evid.QuestionID, desc, url)
	}
	return nil
}

func (c *launchCLI) listAnswerHistory(asset string) error {
	a, err := c.data.RetrieveAssetByID(c.ctx, asset)
	if err != nil {
		return err
	}
	if a.LastLaunchID == nil || *a.LastLaunchID == "" {
		return fmt.Errorf("asset %s has not been launched", asset)
	}
	history, err := c.data.RetrieveUserAnswerHistoryByLaunchID(c.ctx, *a.LastLaunchID)
	if err != nil {
		return err
	}
	if len(history) == 0 {
		fmt.Println("  No answer changes recorded")
		return nil
	}
	fmt.Printf("  Answer change history for launch %s:\n", *a.LastLaunchID)
	for _, h := range history {
		printHistoryEntry(h)
	}
	return nil
}

func (c *launchCLI) listShieldAnswerHistory(asset string) error {
	a, err := c.data.RetrieveAssetByID(c.ctx, asset)
	if err != nil {
		return err
	}
	if a.LastShieldID == nil || *a.LastShieldID == "" {
		return fmt.Errorf("asset %s has no shield", asset)
	}
	history, err := c.data.RetrieveUserAnswerHistoryByShieldID(c.ctx, *a.LastShieldID)
	if err != nil {
		return err
	}
	if len(history) == 0 {
		fmt.Println("  No answer changes recorded")
		return nil
	}
	fmt.Printf("  Answer change history for shield %s:\n", *a.LastShieldID)
	for _, h := range history {
		printHistoryEntry(h)
	}
	return nil
}

func printHistoryEntry(h model.UserAnswerHistory) {
	prev := "-"
	if h.PreviousPredefinedAnswerID != nil {
		prev = *h.PreviousPredefinedAnswerID
	}
	next := h.NewPredefinedAnswerID
	if next == "" {
		next = "-"
	}
	fmt.Printf("    %-26s %-30s → %-30s changed by %-20s at %s\n", h.QuestionID, prev, next, h.ChangedBy, h.ChangedAt)
}

// Steps

func (c *launchCLI) start(asset string) error {
	if err := c.flow.InitiateLaunch(c.ctx, asset); err != nil {
		return err
	}
	fmt.Printf("→ started launch %s\n", workflows.LaunchWorkflowID(asset))
	return nil
}

func (c *launchCLI) submit(asset string, answers []model.AnswerSubmission) error {
	if err := c.flow.SubmitLaunchAnswers(c.ctx, asset, answers); err != nil {
		return err
	}
	fmt.Printf("→ submitted %d answer(s)\n", len(answers))
	return nil
}

func (c *launchCLI) finalize(asset string, answers []model.AnswerSubmission) error {
	if err := c.flow.FinalizeLaunchAnswers(c.ctx, asset, answers); err != nil {
		return err
	}
	fmt.Printf("→ finalized with %d more answer(s)\n", len(answers))
	return nil
}

func (c *launchCLI) validate(asset, tier string) error {
	if err := c.flow.ValidateTier(c.ctx, asset, tier); err != nil {
		return err
	}
	if tier == "" {
		fmt.Println("→ validated the calculated tier")
	} else {
		fmt.Printf("→ validated with tier override %s\n", tier)
	}
	return nil
}

func (c *launchCLI) query(asset string) (workflows.LaunchResult, error) {
	r, err := c.queryState(asset)
	if err != nil {
		return r, err
	}
	printResult("state", r)
	return r, nil
}

func (c *launchCLI) queryState(asset string) (workflows.LaunchResult, error) {
	var r workflows.LaunchResult
	val, err := c.client.QueryWorkflow(c.ctx, workflows.LaunchWorkflowID(asset), "", workflows.LaunchStateQueryName)
	if err != nil {
		return r, err
	}
	err = val.Get(&r)
	return r, err
}

func (c *launchCLI) wait(asset string) (workflows.LaunchResult, error) {
	var r workflows.LaunchResult
	if err := c.client.GetWorkflow(c.ctx, workflows.LaunchWorkflowID(asset), "").Get(c.ctx, &r); err != nil {
		return r, err
	}
	printResult("result", r)
	return r, c.checkPersisted(r)
}

// Shield commands

func (c *launchCLI) shieldStart(asset string) error {
	if err := c.flow.InitiateShield(c.ctx, asset); err != nil {
		return err
	}
	fmt.Printf("→ started shield %s\n", workflows.ShieldWorkflowID(asset))
	return nil
}

func (c *launchCLI) shieldQuery(asset string) (workflows.ShieldResult, error) {
	r, err := c.queryShieldState(asset)
	if err != nil {
		return r, err
	}
	printShieldResult("state", r)
	return r, nil
}

func (c *launchCLI) shieldSubmitAnswers(asset string, answers []model.AnswerSubmission) error {
	if err := c.flow.SubmitShieldAnswers(c.ctx, asset, answers); err != nil {
		return err
	}
	fmt.Printf("→ submitted %d shield answer(s)\n", len(answers))
	return nil
}

func (c *launchCLI) shieldFinalizeAnswers(asset string, answers []model.AnswerSubmission) error {
	if err := c.flow.FinalizeShieldAnswers(c.ctx, asset, answers); err != nil {
		return err
	}
	fmt.Printf("→ finalized shield answers with %d more answer(s)\n", len(answers))
	return nil
}

func (c *launchCLI) shieldSubmitEvidences(asset string, evidences []model.EvidenceSubmission) error {
	if err := c.flow.SubmitShieldEvidences(c.ctx, asset, evidences); err != nil {
		return err
	}
	fmt.Printf("→ submitted %d evidence(s)\n", len(evidences))
	return nil
}

func (c *launchCLI) shieldFinalizeEvidences(asset string, evidences []model.EvidenceSubmission) error {
	if err := c.flow.FinalizeShieldEvidences(c.ctx, asset, evidences); err != nil {
		return err
	}
	fmt.Printf("→ finalized evidences with %d more evidence(s)\n", len(evidences))
	return nil
}

func (c *launchCLI) shieldWait(asset string) (workflows.ShieldResult, error) {
	var r workflows.ShieldResult
	if err := c.client.GetWorkflow(c.ctx, workflows.ShieldWorkflowID(asset), "").Get(c.ctx, &r); err != nil {
		return r, err
	}
	printShieldResult("result", r)
	return r, c.checkShieldPersisted(r)
}

func (c *launchCLI) queryShieldState(asset string) (workflows.ShieldResult, error) {
	var r workflows.ShieldResult
	val, err := c.client.QueryWorkflow(c.ctx, workflows.ShieldWorkflowID(asset), "", workflows.ShieldStateQueryName)
	if err != nil {
		return r, err
	}
	err = val.Get(&r)
	return r, err
}

func (c *launchCLI) waitForShieldState(asset string, state model.ShieldState) (workflows.ShieldResult, error) {
	return c.waitForShield(asset, "state "+string(state), func(r workflows.ShieldResult) bool { return r.State == state })
}

func (c *launchCLI) waitForShield(asset, what string, ok func(workflows.ShieldResult) bool) (workflows.ShieldResult, error) {
	var r workflows.ShieldResult
	err := poll(c.ctx, func() (bool, error) {
		var err error
		r, err = c.queryShieldState(asset)
		return err == nil && ok(r), err
	})
	if err != nil {
		printShieldResult("last state", r)
		return r, fmt.Errorf("waiting for %s: %w", what, err)
	}
	fmt.Printf("  ✓ %s\n", what)
	return r, nil
}

func (c *launchCLI) waitForState(asset string, state model.LaunchState) (workflows.LaunchResult, error) {
	return c.waitFor(asset, "state "+string(state), func(r workflows.LaunchResult) bool { return r.State == state })
}

// waitFor polls the workflow's state until ok returns true. Signals are processed
// asynchronously, so a query right after a signal may not reflect it yet.
func (c *launchCLI) waitFor(asset, what string, ok func(workflows.LaunchResult) bool) (workflows.LaunchResult, error) {
	var r workflows.LaunchResult
	err := poll(c.ctx, func() (bool, error) {
		var err error
		r, err = c.queryState(asset)
		return err == nil && ok(r), err
	})
	if err != nil {
		printResult("last state", r)
		return r, fmt.Errorf("waiting for %s: %w", what, err)
	}
	fmt.Printf("  ✓ %s\n", what)
	return r, nil
}

// Database checks

// waitForPersisted polls the data service until the launch row has the given state and tier.
// The workflow persists between steps asynchronously, so the row can lag the query state.
func (c *launchCLI) waitForPersisted(launchID string, state model.LaunchState, tierID string) error {
	var l model.Launch
	err := poll(c.ctx, func() (bool, error) {
		var err error
		l, err = c.data.RetrieveLaunchByID(c.ctx, launchID)
		return err == nil && l.State == state && strVal(l.TierID) == tierID, err
	})
	if err != nil {
		return fmt.Errorf("persisted launch state=%s tier=%s, want %s/%s: %w", l.State, strVal(l.TierID), state, tierID, err)
	}
	fmt.Printf("  ✓ persisted: state=%s tier=%s\n", l.State, l.TierID)
	return nil
}

// checkShieldPersisted verifies the data service holds the completed shield's final state.
func (c *launchCLI) checkShieldPersisted(r workflows.ShieldResult) error {
	s, err := c.data.RetrieveShieldByID(c.ctx, r.ID)
	if err != nil {
		return fmt.Errorf("shield %s not persisted: %w", r.ID, err)
	}
	if s.AssetID != r.AssetID || s.State != r.State || s.Score != r.Score ||
		s.Reminders != r.Reminders || s.Warnings != r.Warnings || s.Escalations != r.Escalations {
		return fmt.Errorf("persisted shield %+v does not match result %+v", s, r)
	}
	fmt.Printf("  ✓ persisted: state=%s score=%d reminders=%d warnings=%d escalations=%d\n",
		s.State, s.Score, s.Reminders, s.Warnings, s.Escalations)
	return nil
}

// checkPersisted verifies the data service holds the completed launch's final state.
func (c *launchCLI) checkPersisted(r workflows.LaunchResult) error {
	l, err := c.data.RetrieveLaunchByID(c.ctx, r.ID)
	if err != nil {
		return fmt.Errorf("launch %s not persisted: %w", r.ID, err)
	}
	if l.AssetID != r.AssetID || l.State != r.State || strVal(l.TierID) != r.TierID ||
		l.Reminders != r.Reminders || l.Warnings != r.Warnings || l.Escalations != r.Escalations {
		return fmt.Errorf("persisted launch %+v does not match result %+v", l, r)
	}
	fmt.Printf("  ✓ persisted: state=%s tier=%s reminders=%d warnings=%d escalations=%d\n",
		l.State, l.TierID, l.Reminders, l.Warnings, l.Escalations)
	return nil
}

// expectStoredAnswers verifies the launch's stored answers are exactly the given ones. Signals
// are processed asynchronously, so it retries for a few seconds.
func (c *launchCLI) expectStoredAnswers(launchID string, want []model.AnswerSubmission) error {
	wantIDs := map[string]string{}
	for _, a := range want {
		wantIDs[a.QuestionID] = sortedJoin(a.PredefinedAnswerIDs)
	}
	err := poll(c.ctx, func() (bool, error) {
		stored, err := c.data.RetrieveUserAnswersByLaunchID(c.ctx, launchID)
		if err != nil {
			return false, err
		}
		gotIDs := map[string]string{}
		for _, a := range stored {
			ids := make([]string, len(a.Selections))
			for i, sel := range a.Selections {
				ids[i] = sel.PredefinedAnswerID
			}
			gotIDs[a.QuestionID] = sortedJoin(ids)
		}
		if len(gotIDs) != len(wantIDs) {
			return false, fmt.Errorf("%d answers stored, want %d", len(gotIDs), len(wantIDs))
		}
		for q, ids := range wantIDs {
			if gotIDs[q] != ids {
				return false, fmt.Errorf("stored answer for %s is %q, want %q", q, gotIDs[q], ids)
			}
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("  ✓ stored answers: %d answered\n", len(want))
	return nil
}

func (c *launchCLI) expectAsset(assetID, launchID, tierID string) error {
	a, err := c.data.RetrieveAssetByID(c.ctx, assetID)
	if err != nil {
		return err
	}
	if err := expect(strVal(a.LastLaunchID) == launchID && strVal(a.TierID) == tierID && a.State == model.PendingShieldAssetState,
		"asset lastLaunch=%s tier=%s state=%s, want %s/%s/%s",
		strVal(a.LastLaunchID), strVal(a.TierID), a.State, launchID, tierID, model.PendingShieldAssetState); err != nil {
		return err
	}
	fmt.Printf("  ✓ asset %s: tier=%s state=%s lastLaunch=%s\n", assetID, strVal(a.TierID), a.State, strVal(a.LastLaunchID))
	return nil
}

// Helpers

func (c *launchCLI) assetOrFirst(args []string) (string, error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], nil
	}
	assets, err := c.data.RetrieveAssets(c.ctx)
	if err != nil {
		return "", err
	}
	if len(assets) == 0 {
		return "", errors.New("no assets in the database")
	}
	return assets[0].ID, nil
}

func (c *launchCLI) workerName(id string) string {
	if id == "" {
		return "-"
	}
	w, err := c.data.RetrieveWorkerByID(c.ctx, id)
	if err != nil {
		return id + "(?)"
	}
	return w.Name
}

// poll calls check every 200ms until it returns true, for up to 10 seconds. The last error
// from check, if any, is returned on timeout.
func poll(ctx context.Context, check func() (bool, error)) error {
	deadline := time.Now().Add(10 * time.Second)
	for {
		ok, err := check()
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return err
			}
			return errors.New("timed out")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func printResult(label string, r workflows.LaunchResult) {
	fmt.Printf("  %s: id=%s asset=%s state=%s points=%d (score %.2f) tier=%s answered=%d/%d reminders=%d warnings=%d escalations=%d running=%s\n",
		label, r.ID, r.AssetID, r.State, r.Points, float64(r.Points)/100, r.TierID, r.Answered, r.Questions, r.Reminders, r.Warnings, r.Escalations,
		r.RunningTime.Round(time.Millisecond))
}

func printShieldResult(label string, r workflows.ShieldResult) {
	fmt.Printf("  %s: id=%s asset=%s state=%s score=%d answered=%d/%d evidences=%d/%d reminders=%d warnings=%d escalations=%d running=%s\n",
		label, r.ID, r.AssetID, r.State, r.Score, r.Answered, r.Questions, r.EvidencesCount, r.EvidenceRequired, r.Reminders, r.Warnings, r.Escalations,
		r.RunningTime.Round(time.Millisecond))
}

func argAt(args []string, i int, name string) (string, error) {
	if i >= len(args) || args[i] == "" {
		return "", fmt.Errorf("missing <%s>", name)
	}
	return args[i], nil
}

// predefinedAnswers returns the predefined answers of the given category type, both per question
// (in display order) and as one list.
func (c *launchCLI) predefinedAnswers(t model.QuestionCategoryType) (map[string][]model.PredefinedAnswer, []model.PredefinedAnswer, error) {
	all, err := c.data.RetrievePredefinedAnswersByCategoryType(c.ctx, t)
	if err != nil {
		return nil, nil, err
	}
	byQuestion := map[string][]model.PredefinedAnswer{}
	for _, a := range all {
		byQuestion[a.QuestionID] = append(byQuestion[a.QuestionID], a)
	}
	return byQuestion, all, nil
}

// pickAnswers answers each question with one predefined answer. pick gets the question's
// position and its number of choices, and returns the index of the choice to select.
func pickAnswers(questions []model.Question, byQuestion map[string][]model.PredefinedAnswer, pick func(i, n int) int) []model.AnswerSubmission {
	answers := make([]model.AnswerSubmission, len(questions))
	for i, q := range questions {
		choices := byQuestion[q.ID]
		answers[i] = model.AnswerSubmission{QuestionID: q.ID, PredefinedAnswerIDs: []string{choices[pick(i, len(choices))].ID}}
	}
	return answers
}

// submissionPoints is the points the submitted answers are worth.
func submissionPoints(submissions []model.AnswerSubmission, predefined []model.PredefinedAnswer) int {
	details := make([]model.UserAnswerDetail, len(submissions))
	for i, sub := range submissions {
		for _, id := range sub.PredefinedAnswerIDs {
			details[i].Selections = append(details[i].Selections, model.UserAnswerSelection{PredefinedAnswerID: id})
		}
	}
	return answers.Points(details, predefined)
}

func sortedJoin(ids []string) string {
	sorted := append([]string{}, ids...)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// parseAnswers turns "q1=a1 q2=a2,a3" into answer submissions.
func parseAnswers(args []string) ([]model.AnswerSubmission, error) {
	submissions := make([]model.AnswerSubmission, 0, len(args))
	for _, a := range args {
		q, ids, ok := strings.Cut(a, "=")
		if !ok || q == "" || ids == "" {
			return nil, fmt.Errorf("invalid answer %q, want questionID=answerID[,answerID…]", a)
		}
		submissions = append(submissions, model.AnswerSubmission{QuestionID: q, PredefinedAnswerIDs: strings.Split(ids, ",")})
	}
	return submissions, nil
}

// parseEvidences turns "q1=desc:url q2=desc2:url2" into evidence submissions.
func parseEvidences(args []string) ([]model.EvidenceSubmission, error) {
	evidences := make([]model.EvidenceSubmission, 0, len(args))
	for _, a := range args {
		questionID, rest, ok := strings.Cut(a, "=")
		if !ok || questionID == "" {
			return nil, fmt.Errorf("invalid evidence %q, want questionID=description:url", a)
		}
		desc, url, ok := strings.Cut(rest, ":")
		if !ok {
			return nil, fmt.Errorf("invalid evidence %q, want questionID=description:url", a)
		}
		evidences = append(evidences, model.EvidenceSubmission{QuestionID: questionID, Description: desc, URL: url})
	}
	return evidences, nil
}

func expect(ok bool, format string, args ...any) error {
	if ok {
		return nil
	}
	return fmt.Errorf(format, args...)
}

// expectErr checks that a step was rejected with want.
func expectErr(err, want error, what string) error {
	if !errors.Is(err, want) {
		return fmt.Errorf("%s: got error %v, want %v", what, err, want)
	}
	fmt.Printf("  ✓ rejected %s: %v\n", what, err)
	return nil
}
