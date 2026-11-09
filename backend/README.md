## Run Locally

```bash
go mod init github.com/khaledhikmat/psap
go get -u go.temporal.io/sdk
go get -u go.temporal.io/api

# terminal 1
temporal server start-dev
CLI 1.5.1 (Server 1.29.1, UI 2.42.1)

Server:  localhost:7233
UI:      http://localhost:8233
Metrics: http://localhost:57058/metrics

# terminal 2 - flow worker
cd backend
MODE_PROC=flowworker go run .
or
MODE_PROC=multiflowworker go run .

# terminal 3 - starter/test CLI. Eventually this an API Layer
MODE_PROC=cli go run . help        # list all commands

# Data commands
MODE_PROC=cli go run . assets                       # list assets
MODE_PROC=cli go run . questions                    # list launch questions with answer choices and points
MODE_PROC=cli go run . shield-questions             # list shield questions with answer choices and points
MODE_PROC=cli go run . tiers                        # list tiers
MODE_PROC=cli go run . answers <assetID>            # list selected launch answers
MODE_PROC=cli go run . shield-answers <assetID>     # list selected shield answers
MODE_PROC=cli go run . evidences <assetID>          # list shield evidences
MODE_PROC=cli go run . answer-history <assetID>     # list launch answer change history (audit trail)
MODE_PROC=cli go run . shield-answer-history <assetID>  # list shield answer change history (audit trail)

# Launch workflow commands
MODE_PROC=cli go run . start <assetID>           # start a launch
MODE_PROC=cli go run . query <assetID>           # query launch state
MODE_PROC=cli go run . submit <assetID> q1=a1 q2=a2,a3  # submit partial answers (question=predefined answer IDs)
MODE_PROC=cli go run . finalize <assetID>        # finalize answers (unanswered earn no points)
MODE_PROC=cli go run . validate <assetID> [tier] # validate tier
MODE_PROC=cli go run . wait <assetID>            # wait for launch completion

# Shield workflow commands
MODE_PROC=cli go run . shield-start <assetID>    # start a shield
MODE_PROC=cli go run . shield-query <assetID>    # query shield state
MODE_PROC=cli go run . shield-submit-answers <assetID> q1=a1 q2=a2     # submit partial answers
MODE_PROC=cli go run . shield-finalize-answers <assetID>                # finalize answers
MODE_PROC=cli go run . shield-submit-evidences <assetID> q1=desc:url  # submit partial evidences (answered questions)
MODE_PROC=cli go run . shield-finalize-evidences <assetID>              # finalize evidences
MODE_PROC=cli go run . shield-wait <assetID>     # wait for shield completion

# Test scenarios
MODE_PROC=cli go run . happy          # launch: start → submit → finalize → validate
MODE_PROC=cli go run . relaunch       # relaunch mid-launch cancels the running launch
MODE_PROC=cli go run . timeout        # reminders/warnings/escalations until timed out (~25s)
MODE_PROC=cli go run . shield-happy   # shield: submit answers → submit evidences → complete

# terminal 4 - this an API Layer
MODE_PROC=httpserver go run . 

# terminal 5 - this is the minion manager
MODE_PROC=minionmanager go run . 
```

## Database Access

```bash
cd backend
sqlite3 psap.db
.tables
.schema
.schema <table-name>
SELECT * FROM ASSETS;
.quit
```

## Scoring

Questions have predefined answers, each worth some points (see [prd.md](../prd.md)).

- **Launch:** every question scores 1–5 in the order its answers appear. The weighted score is
  Q3×0.20 + Q5×0.25 + Q6×0.20 + Q7×0.25 + Q8×0.10 (Q1, Q2 and Q4 carry no weight). Answer points are
  score × weight in percent, so a launch's points are its score in hundredths (e.g. 205 points = 2.05)
  and tiers are ranges of those points: Tier 1 400–500, Tier 2 300–399, Tier 3 200–299, Tier 4 0–199.
  Unanswered questions earn no points. Non-Tier is never calculated; the assessor assigns it when
  validating the launch.
- **Shield:** nine single-choice questions, each scored 1–5 (5 = highest exposure) and each requiring
  evidence. The shield score is the sum (9–45). Answers can only be finalized when every question is
  answered, and evidence only when every question has some.

Seed data lives in `service/data/seed.go`; it is only loaded into empty tables, so delete `psap.db` to
re-seed after changing it.

## API Endpoints

Base URL: `http://localhost:8080/v1/physec/accesscontrol/psap`

### Health Check
```
GET  /ping
```

### Tiers
```
GET  /tiers           # List all tiers
GET  /tiers/:id       # Get tier by ID
```

### Question Categories
```
GET  /question-categories              # List all categories (filter by ?type=launch or ?type=shield)
GET  /question-categories/:id          # Get category by ID
GET  /question-categories/:id/questions # List questions in category
```

### Questions
```
GET  /questions       # List all questions (filter by ?type=launch or ?type=shield)
GET  /questions/:id   # Get question by ID
GET  /questions/:id/answers  # List the question's predefined answers (choices and points)
GET  /predefined-answers     # List all predefined answers (filter by ?type=launch or ?type=shield)
```

### Workers
```
GET  /workers         # List all workers
GET  /workers/:id     # Get worker by ID
```

### Assets
```
GET  /assets              # List all assets
POST /assets              # Create an asset request (starts in pending-launch)
GET  /assets/:id          # Get asset by ID
GET  /assets/:id/launches # List asset's launches
GET  /assets/:id/shields  # List asset's shields
GET  /assets/:id/baselines # List asset's baselines
```

**POST /assets**
```json
{
  "name": "North Campus Archive",
  "campus": "North Campus",
  "building": "A2",
  "floor": "2",
  "space": "ARC-02",
  "priority": "normal",
  "businessOwnerId": "worker-owner-1",
  "businessManagerId": "worker-manager-1"
}
```

The owner and manager must be existing workers. Creating the asset does not start a launch;
the client can start one with `POST /launches` after the request is saved.

### Launches
```
GET  /launches                       # List all launches
GET  /launches/:id                   # Get launch by ID
GET  /launches/:id/answers           # List launch answers with their selected predefined answers
GET  /launches/:id/history           # List answer change history (audit trail)
POST /launches                       # Initiate a new launch
POST /launches/:id/submit-answers    # Submit partial answers
POST /launches/:id/finalize-answers  # Finalize answers
POST /launches/:id/validate-tier     # Validate tier
```

### Shields
```
GET  /shields                          # List all shields
GET  /shields/:id                      # Get shield by ID
GET  /shields/:id/answers              # List shield answers with their selected predefined answers
GET  /shields/:id/evidences            # List shield evidences
GET  /shields/:id/history              # List answer change history (audit trail)
POST /shields                          # Initiate a new shield
POST /shields/:id/submit-answers       # Submit partial answers
POST /shields/:id/finalize-answers     # Finalize answers
POST /shields/:id/submit-evidences     # Submit partial evidences
POST /shields/:id/finalize-evidences   # Finalize evidences
```

### Request/Response Examples

**POST /launches**
```json
{
  "assetId": "asset-hq-dc"
}
```

**POST /launches/:id/submit-answers** (same body for `/shields/:id/submit-answers`)

Each answer selects predefined answer IDs. Several IDs are only accepted for questions with
`allowMultipleAnswers`. Invalid submissions are rejected with `400`.
```json
{
  "answers": [
    {"questionId": "launch-room-function-1", "predefinedAnswerIds": ["launch-room-function-1-1"]},
    {"questionId": "launch-asset-type-1", "predefinedAnswerIds": ["launch-asset-type-1-3"]}
  ]
}
```

**POST /shields/:id/submit-evidences**

Evidence is recorded for questions that already have an answer.
```json
{
  "evidences": [
    {"questionId": "shield-perimeter-1", "description": "Security audit report", "url": "https://example.com/audit.pdf"},
    {"questionId": "shield-perimeter-2", "description": "Compliance certification", "url": "https://example.com/cert.pdf"}
  ]
}
```

