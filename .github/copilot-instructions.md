# PSAP 2.0: project notes for Copilot

Go backend in `backend/` (Temporal workflows + SQLite + Gin API + a test CLI); product spec in `prd.md`.
Status below was last updated 2026-11-02. Keep this file current when the work moves on.

## Status: backend migrated to the new answer model (uncommitted)

`backend/model/model.go` is owned by the user. It defines Question flags (`AllowMultipleAnswers`, `EvidenceRequired`,
`DisplayOrder`), `PredefinedAnswer`, `UserAnswer`, `UserAnswerSelection`, `UserAnswerHistory` and a new `Evidence`.
The whole backend was adapted to it and then to the completed launch and shield sections of `prd.md`. The legacy
`Answer`/`AnswerHistory`/YES-NO code was removed.

### Where things live
- `model/model.go`: user's types plus my `UserAnswerDetail`, `AnswerSubmission{QuestionID, PredefinedAnswerIDs}`,
  `EvidenceSubmission{QuestionID, Description, URL}`.
- `service/answers/`: shared `Validate` (known question, answer belongs to the question, no duplicates, several answers only
  if `AllowMultipleAnswers`), `Points`, `Answered`, `ErrInvalid`.
- `service/data/`: tables PREDEFINED_ANSWERS, USER_ANSWERS, USER_ANSWER_SELECTIONS, USER_ANSWER_HISTORY.
  `SaveLaunchAnswers`/`SaveShieldAnswers` run in one transaction, are idempotent and write one history row per change.
  `seed.go` holds the seed built from `prd.md`. Test: `answers_test.go`.
- `service/flow/activities/{launch,shield}`, `service/flow/workflows/`, `service/flow/temporal.go`. Answers and evidence
  are created lazily on first submission. Shield activities are named `CreateShield`, `SaveShieldAnswers`, `PersistShield`
  because Temporal registers activities by method name and they collided with launch's.
- `mode/flowworker.go` and `mode/multiflowworker.go` register both the Launch and Shield workflows (shield was never registered before).
- `api/`: invalid or incomplete submissions return 400. Endpoints `GET /questions/:id/answers` and
  `GET /predefined-answers[?type=]` were added. `backend/README.md` documents endpoints and scoring.
- `mode/cli.go`: answers as `questionID=answerID[,answerID]`, evidence as `questionID=desc:url`. Scenarios `happy`,
  `relaunch`, `shield-happy` (and the unchanged `timeout`) verify validation, history, scores and the completeness rules.

### Scoring model (from `prd.md`)
- **Launch:** 8 categories, one question each. Answers score 1-5 in listed order. Weighted score =
  Q3x0.20 + Q5x0.25 + Q6x0.20 + Q7x0.25 + Q8x0.10; Q1 (room function, multi-select), Q2 (regulatory) and Q4 (routine access)
  carry no weight and have 0 points. Answer points = score x weight %, so a launch's points are its score in HUNDREDTHS
  (205 = 2.05). Tiers are ranges of those points: T1 400-500, T2 300-399, T3 200-299, T4 0-199 (0 is only reachable with
  unanswered questions). `non-tier` (-1..-1) is never calculated; the assessor assigns it through the tier override when validating.
- **Shield:** 9 single-choice questions scored 1-5, all `EvidenceRequired`. Score = plain sum (9-45).
- **Shield finalize rules** (checked synchronously in the flow service): finalizing answers is rejected until all 9 are
  answered; finalizing evidence is rejected until every required question has evidence. Launch may still finalize with
  unanswered questions (they earn 0 points).

### Gotchas
- Seeding only fills EMPTY tables. After changing `seed.go`, delete `backend/psap.db*` (gitignored).
- The signal payload shapes changed, so Temporal workflows still running from an older version would break.
- Temporal registers activities by method name; keep names unique across launch, shield and notification activities.

## Frontend status (2026-11-02)

- `frontend/` is a shared React application with `/business-owner` and `/business-manager` routes.
- The demo actor picker selects assigned workers and changes portal routes; it is not authentication or authorization.
- Owners can request assets and submit LAUNCH answers. Managers can validate submitted LAUNCH tiers and start/continue SHIELD,
  including recording required evidence. Both roles can view historic LAUNCH and SHIELD assessments read-only.
- While a LAUNCH waits for tier approval, managers may revise the owner's answers: `POST /launches/:id/revise-answers` sends a
  Temporal Update (`launch-revise-answers-update`) that saves the changes with the manager as `ChangedBy`, recalculates the tier
  and returns it synchronously. Changes appear in `GET /launches/:id/history`. Restart workers after changing workflow code.
- "Start SHIELD" asks for confirmation when a SHIELD is already running; confirming terminates it and starts a new one.
- New manager tier approvals have a seven-day deadline beginning when the tier is calculated; expired launches remain visible
  in the manager queue as read-only records.
- Admins (`/admin`, workers whose ID starts with `worker-admin`; one is seeded, and added to older databases on startup) get a
  read-only portal: searchable assets, LAUNCHes and SHIELDs (by location, owner, manager) sorted by last update. They open the
  asset detail history but have no actions.
- Security Assessor and Security Implementer portals are not implemented. Baselines and compliance/investment workflows remain open.
- Verify frontend changes with `npm --prefix frontend run build`.

## How to verify
- `cd backend && go build ./... && go vet ./... && go test ./service/data/ ./service/flow/...`. Vet prints 2 old warnings in `mode/cli.go`
  (`%s` with a `*string` TierID); they are pre-existing and were left alone.
- End to end: the user normally has a Temporal dev server on :7233 (the Go clients dial the default address, so they cannot
  use another port). Check it is idle with `temporal workflow list`. Run the worker with
  `SQLITE_FILE=/tmp/psap-test.db MODE_PROC=flowworker go run .`, then `MODE_PROC=cli go run . <happy|relaunch|shield-happy> <assetID>`
  with the same `SQLITE_FILE`. Test assets: `asset-hq-dc`, `asset-rd-lab`. Afterwards stop the worker, terminate leftover
  workflows (`temporal workflow terminate --workflow-id asset-launch-wf-<asset>`), and delete the temp DB. Never use the user's `psap.db` for tests.

## Open items / next steps
1. Confirm my interpretations: "Backup / Not applicable" scores like "Full redundancy" (lowest); the shield score is a plain sum;
   launch finalize with unanswered questions stays allowed (unlike shield).
2. Seed text: trailing `?` removed from some answers, typos fixed (upports, ompact, Modederate), "workshop" kept, shield title "Recog" kept as written. Confirm.
3. Not done: launch-side evidence (the data layer supports it, no workflow uses it); PRD steps 5-6 (baseline, compliance, action items);
   an endpoint for a single user answer; Security Assessor and Security Implementer portals.
4. Known unfixed issue: CLI prints `%!s(*string=...)` for `l.TierID`.
5. Nothing is committed. When committing, add the trailer `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`.
   Untracked: `prd.md`, `backend/service/answers/`, `backend/service/data/answers_test.go`.
