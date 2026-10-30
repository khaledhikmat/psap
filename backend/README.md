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

# terminal 3 - starter/test CLI. Eventually this an API Layer
MODE_PROC=cli go run . help        # list commands
MODE_PROC=cli go run . happy       # start → submit → finalize → validate
MODE_PROC=cli go run . relaunch    # relaunch mid-launch cancels the running launch
MODE_PROC=cli go run . timeout     # reminders/warnings/escalations until timed out (~25s)
```

