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