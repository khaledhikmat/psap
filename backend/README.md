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
MODE_PROC=flowworker go run .
or
MODE_PROC=multiflowworker go run .

# terminal 3 - starter/test CLI. Eventually this an API Layer
MODE_PROC=cli go run . help        # list all commands

# Data commands
MODE_PROC=cli go run . assets                       # list assets
MODE_PROC=cli go run . questions                    # list launch questions with weights
MODE_PROC=cli go run . shield-questions             # list shield questions with weights
MODE_PROC=cli go run . tiers                        # list tiers
MODE_PROC=cli go run . answers <assetID>            # list launch answers
MODE_PROC=cli go run . shield-answers <assetID>     # list shield answers
MODE_PROC=cli go run . evidences <assetID>          # list shield evidences
MODE_PROC=cli go run . answer-history <assetID>     # list launch answer change history (audit trail)
MODE_PROC=cli go run . shield-answer-history <assetID>  # list shield answer change history (audit trail)

# Launch workflow commands
MODE_PROC=cli go run . start <assetID>           # start a launch
MODE_PROC=cli go run . query <assetID>           # query launch state
MODE_PROC=cli go run . submit <assetID> q1=yes q2=no  # submit partial answers
MODE_PROC=cli go run . finalize <assetID>        # finalize answers (unanswered become NO)
MODE_PROC=cli go run . validate <assetID> [tier] # validate tier
MODE_PROC=cli go run . wait <assetID>            # wait for launch completion

# Shield workflow commands
MODE_PROC=cli go run . shield-start <assetID>    # start a shield
MODE_PROC=cli go run . shield-query <assetID>    # query shield state
MODE_PROC=cli go run . shield-submit-answers <assetID> q1=yes q2=no     # submit partial answers
MODE_PROC=cli go run . shield-finalize-answers <assetID>                # finalize answers
MODE_PROC=cli go run . shield-submit-evidences <assetID> cat1=desc:url  # submit partial evidences
MODE_PROC=cli go run . shield-finalize-evidences <assetID>              # finalize evidences
MODE_PROC=cli go run . shield-wait <assetID>     # wait for shield completion

# Test scenarios
MODE_PROC=cli go run . happy          # launch: start → submit → finalize → validate
MODE_PROC=cli go run . relaunch       # relaunch mid-launch cancels the running launch
MODE_PROC=cli go run . timeout        # reminders/warnings/escalations until timed out (~25s)
MODE_PROC=cli go run . shield-happy   # shield: submit answers → submit evidences → complete
```

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
GET  /questions       # List all questions (filter by ?categoryType=launch or ?categoryType=shield)
GET  /questions/:id   # Get question by ID
```

### Workers
```
GET  /workers         # List all workers
GET  /workers/:id     # Get worker by ID
```

### Assets
```
GET  /assets              # List all assets
GET  /assets/:id          # Get asset by ID
GET  /assets/:id/launches # List asset's launches
GET  /assets/:id/shields  # List asset's shields
GET  /assets/:id/baselines # List asset's baselines
```

### Launches
```
GET  /launches                       # List all launches
GET  /launches/:id                   # Get launch by ID
GET  /launches/:id/answers           # List launch answers
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
GET  /shields/:id/answers              # List shield answers
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

**POST /launches/:id/submit-answers**
```json
{
  "answers": [
    {"questionId": "question-security-1", "reply": "YES"},
    {"questionId": "question-compliance-2", "reply": "NO"}
  ]
}
```

**POST /shields/:id/submit-evidences**
```json
{
  "evidences": {
    "category-security": {
      "description": "Security audit report",
      "url": "https://example.com/audit.pdf"
    },
    "category-compliance": {
      "description": "Compliance certification",
      "url": "https://example.com/cert.pdf"
    }
  }
}
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