## Run Locally

```bash
go mod init github.com/khaledhikmat/psap
go get -u go.temporal.io/sdk
go get -u go.temporal.io/api

# terminal 1 - HTTP server
cd backend
go run .
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

