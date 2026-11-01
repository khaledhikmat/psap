package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/config"
	"github.com/khaledhikmat/psap/service/lgr"
	"golang.org/x/xerrors"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Tables are derived from the models' `db` tags, so the model structs are the single source
// of truth for the schema. Columns added to a model are added to its table on startup.
var (
	workersTable            = newTable[model.Worker]("WORKERS", "ID")
	assetsTable             = newTable[model.Asset]("ASSETS", "ASSET_ID")
	questionCategoriesTable = newTable[model.QuestionCategory]("QUESTION_CATEGORIES", "ID")
	questionsTable          = newTable[model.Question]("QUESTIONS", "ID")
	predefinedAnswersTable  = newTable[model.PredefinedAnswer]("PREDEFINED_ANSWERS", "ID")
	userAnswersTable        = newTable[model.UserAnswer]("USER_ANSWERS", "ID")
	userSelectionsTable     = newTable[model.UserAnswerSelection]("USER_ANSWER_SELECTIONS", "ID")
	userAnswerHistoryTable  = newTable[model.UserAnswerHistory]("USER_ANSWER_HISTORY", "ID")
	launchesTable           = newTable[model.Launch]("LAUNCHES", "ID")
	shieldsTable            = newTable[model.Shield]("SHIELDS", "ID")
	evidencesTable          = newTable[model.Evidence]("EVIDENCES", "ID")
	baselinesTable          = newTable[model.Baseline]("BASELINES", "ID")
	actionItemsTable        = newTable[model.ActionItem]("ACTION_ITEMS", "ID")
	tiersTable              = newTable[model.Tier]("TIERS", "ID")

	allTables = []table{
		workersTable, assetsTable, questionCategoriesTable, questionsTable, predefinedAnswersTable,
		userAnswersTable, userSelectionsTable, userAnswerHistoryTable, launchesTable, shieldsTable, evidencesTable, baselinesTable, actionItemsTable, tiersTable,
	}
)

var schemaExtras = []string{
	`CREATE TABLE IF NOT EXISTS EVENTS (
		ID INTEGER PRIMARY KEY AUTOINCREMENT,
		CREATED_AT TEXT NOT NULL,
		PAYLOAD TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS ASSETS_NAME_IDX ON ASSETS (ASSET_NAME)`,
	`CREATE INDEX IF NOT EXISTS QUESTION_CATEGORIES_TYPE_IDX ON QUESTION_CATEGORIES (TYPE)`,
	`CREATE INDEX IF NOT EXISTS QUESTIONS_CATEGORY_IDX ON QUESTIONS (CATEGORY_ID)`,
	`CREATE INDEX IF NOT EXISTS PREDEFINED_ANSWERS_QUESTION_IDX ON PREDEFINED_ANSWERS (QUESTION_ID)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS USER_ANSWERS_LAUNCH_QUESTION_IDX ON USER_ANSWERS (LAUNCH_ID, QUESTION_ID) WHERE LAUNCH_ID IS NOT NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS USER_ANSWERS_SHIELD_QUESTION_IDX ON USER_ANSWERS (SHIELD_ID, QUESTION_ID) WHERE SHIELD_ID IS NOT NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS USER_ANSWER_SELECTIONS_UNIQUE_IDX ON USER_ANSWER_SELECTIONS (USER_ANSWER_ID, PREDEFINED_ANSWER_ID)`,
	`CREATE INDEX IF NOT EXISTS USER_ANSWER_HISTORY_ANSWER_IDX ON USER_ANSWER_HISTORY (USER_ANSWER_ID)`,
	`CREATE INDEX IF NOT EXISTS USER_ANSWER_HISTORY_LAUNCH_IDX ON USER_ANSWER_HISTORY (LAUNCH_ID) WHERE LAUNCH_ID IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS USER_ANSWER_HISTORY_SHIELD_IDX ON USER_ANSWER_HISTORY (SHIELD_ID) WHERE SHIELD_ID IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS LAUNCHES_ASSET_IDX ON LAUNCHES (ASSET_ID)`,
	`CREATE INDEX IF NOT EXISTS SHIELDS_ASSET_IDX ON SHIELDS (ASSET_ID)`,
	`CREATE INDEX IF NOT EXISTS EVIDENCES_LAUNCH_IDX ON EVIDENCES (LAUNCH_ID) WHERE LAUNCH_ID IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS EVIDENCES_SHIELD_IDX ON EVIDENCES (SHIELD_ID) WHERE SHIELD_ID IS NOT NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS EVIDENCES_USER_ANSWER_IDX ON EVIDENCES (USER_ANSWER_ID)`,
	`CREATE INDEX IF NOT EXISTS BASELINES_ASSET_IDX ON BASELINES (ASSET_ID)`,
	`CREATE INDEX IF NOT EXISTS ACTION_ITEMS_BASELINE_IDX ON ACTION_ITEMS (BASELINE_ID)`,
	`CREATE INDEX IF NOT EXISTS TIERS_POINTS_IDX ON TIERS (FROM_POINT, TO_POINT)`,
}

type sqliteService struct {
	db *sqlx.DB
}

// NewSQLLite opens (creating if needed) the SQLite database named by the config service,
// brings its schema up to date and seeds empty tables. Call Finalize to close the database.
func NewSQLLite(canxCtx context.Context, cfgSvc config.IService) IService {
	file := "psap.db"
	if cfgSvc != nil && cfgSvc.SQLiteFile() != "" {
		file = cfgSvc.SQLiteFile()
	}

	// WAL + busy_timeout let several processes (http server, workers, CLI) share the file.
	dsn := "file:" + file + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sqlx.Open("sqlite", dsn)
	if err == nil {
		err = migrate(canxCtx, db)
	}
	if err == nil {
		err = seed(canxCtx, db)
	}
	if err != nil {
		lgr.Logger.Error(
			"unable to open SQLite database",
			slog.String("file", file),
			slog.Any("error", xerrors.New(err.Error())),
		)
		if db != nil {
			_ = db.Close()
		}
		return nil
	}

	lgr.Logger.Info("SQLite data service ready", slog.String("file", file))
	return &sqliteService{db: db}
}

func (s *sqliteService) Finalize() {
	if err := s.db.Close(); err != nil {
		lgr.Logger.Error(
			"unable to close SQLite database",
			slog.Any("error", xerrors.New(err.Error())),
		)
	}
}

func migrate(ctx context.Context, db *sqlx.DB) error {
	for _, t := range allTables {
		if _, err := db.ExecContext(ctx, t.createSQL()); err != nil {
			return fmt.Errorf("create table %s: %w", t.name, err)
		}

		// Add columns that were added to the model after the table was created.
		var existing []string
		if err := db.SelectContext(ctx, &existing, `SELECT name FROM pragma_table_info(?)`, t.name); err != nil {
			return fmt.Errorf("inspect table %s: %w", t.name, err)
		}
		have := map[string]bool{}
		for _, c := range existing {
			have[strings.ToUpper(c)] = true
		}
		for _, c := range t.cols {
			if have[c.name] {
				continue
			}
			if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s`, t.name, t.columnDef(c))); err != nil {
				return fmt.Errorf("add column %s.%s: %w", t.name, c.name, err)
			}
		}
	}

	for _, stmt := range schemaExtras {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// withTx runs fn in a transaction, committing if it returns nil and rolling back otherwise.
func (s *sqliteService) withTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// NewEvent stores an arbitrary event as JSON.
func (s *sqliteService) NewEvent(ctx context.Context, evt interface{}) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO EVENTS (CREATED_AT, PAYLOAD) VALUES (?, ?)`, now(), string(payload))
	return err
}

// Generic table access

// dbtx is satisfied by both *sqlx.DB and *sqlx.Tx, so the helpers below work in and out of
// transactions.
type dbtx interface {
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

type column struct {
	name     string
	sqlType  string
	index    int  // struct field index
	nullable bool // true for pointer types (*string, *int, etc.)
}

type table struct {
	name   string
	id     string
	cols   []column
	byName map[string]column
}

func newTable[T any](name, id string) table {
	t := table{name: name, id: id, byName: map[string]column{}}
	typ := reflect.TypeFor[T]()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := f.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}

		// Check if the field is a pointer (nullable)
		fieldType := f.Type
		nullable := false
		if fieldType.Kind() == reflect.Ptr {
			nullable = true
			fieldType = fieldType.Elem() // get the underlying type
		}

		sqlType := "TEXT"
		switch fieldType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Bool:
			sqlType = "INTEGER"
		case reflect.Float32, reflect.Float64:
			sqlType = "REAL"
		}
		c := column{name: tag, sqlType: sqlType, index: i, nullable: nullable}
		t.cols = append(t.cols, c)
		t.byName[tag] = c
	}
	if _, ok := t.byName[id]; !ok {
		panic(fmt.Sprintf("data: table %s has no %s column", name, id))
	}
	return t
}

func (t table) createSQL() string {
	defs := make([]string, 0, len(t.cols))
	for _, c := range t.cols {
		defs = append(defs, t.columnDef(c))
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n\t%s\n)", t.name, strings.Join(defs, ",\n\t"))
}

// columnDef gives every non-ID column a zero-value default, so existing rows get a
// scannable value when a column is added later. Nullable columns (pointer types) allow NULL.
func (t table) columnDef(c column) string {
	switch {
	case c.name == t.id:
		return c.name + " " + c.sqlType + " PRIMARY KEY NOT NULL"
	case c.nullable:
		return c.name + " " + c.sqlType // nullable, no NOT NULL or DEFAULT
	case c.sqlType == "TEXT":
		return c.name + " TEXT NOT NULL DEFAULT ''"
	default:
		return c.name + " " + c.sqlType + " NOT NULL DEFAULT 0"
	}
}

// columnList lists the table's columns, each prefixed with alias (e.g. "q.") when given.
func (t table) columnList(alias string) string {
	names := make([]string, len(t.cols))
	for i, c := range t.cols {
		names[i] = alias + c.name
	}
	return strings.Join(names, ", ")
}

// field returns the struct field behind a column, if the table has that column.
func (t table) field(v reflect.Value, col string) (reflect.Value, bool) {
	c, ok := t.byName[col]
	if !ok {
		return reflect.Value{}, false
	}
	return v.Elem().Field(c.index), true
}

func insertRow[T any](ctx context.Context, q dbtx, t table, row T) error {
	v := reflect.ValueOf(&row)
	id, _ := t.field(v, t.id)
	if id.String() == "" {
		return fmt.Errorf("%s: missing %s", t.name, t.id)
	}
	ts := now()
	for _, col := range []string{"CREATED_AT", "UPDATED_AT"} {
		if f, ok := t.field(v, col); ok && f.String() == "" {
			f.SetString(ts)
		}
	}

	names := make([]string, len(t.cols))
	params := make([]string, len(t.cols))
	for i, c := range t.cols {
		names[i] = c.name
		params[i] = ":" + c.name
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", t.name, strings.Join(names, ", "), strings.Join(params, ", "))
	_, err := q.NamedExecContext(ctx, query, row)
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) &&
		(sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY || sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE) {
		return fmt.Errorf("%s %s: %w", t.name, id.String(), ErrAlreadyExists)
	}
	return err
}

// updateRow overwrites every column except the ID and creation fields, and stamps UPDATED_AT.
func updateRow[T any](ctx context.Context, q dbtx, t table, row T) error {
	v := reflect.ValueOf(&row)
	id, _ := t.field(v, t.id)
	if id.String() == "" {
		return fmt.Errorf("%s: missing %s", t.name, t.id)
	}
	if f, ok := t.field(v, "UPDATED_AT"); ok {
		f.SetString(now())
	}

	sets := make([]string, 0, len(t.cols))
	for _, c := range t.cols {
		if c.name == t.id || c.name == "CREATED_AT" || c.name == "CREATED_BY" {
			continue
		}
		sets = append(sets, c.name+" = :"+c.name)
	}
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = :%s", t.name, strings.Join(sets, ", "), t.id, t.id)
	res, err := q.NamedExecContext(ctx, query, row)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("%s %s: %w", t.name, id.String(), ErrNotFound)
	}
	return nil
}

// getRow returns the first row matching where (e.g. "ID = ?"), in table order.
func getRow[T any](ctx context.Context, q dbtx, t table, where string, args ...any) (T, error) {
	var row T
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s%s LIMIT 1", t.columnList(""), t.name, where, t.orderBy(""))
	err := q.GetContext(ctx, &row, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return row, fmt.Errorf("%s %v: %w", t.name, args, ErrNotFound)
	}
	return row, err
}

// listRows returns the rows matching where, or all rows when where is empty, oldest first.
// It never returns a nil slice, so empty results encode as [] in JSON.
func listRows[T any](ctx context.Context, q dbtx, t table, where string, args ...any) ([]T, error) {
	rows := []T{}
	query := fmt.Sprintf("SELECT %s FROM %s", t.columnList(""), t.name)
	if where != "" {
		query += " WHERE " + where
	}
	query += t.orderBy("")
	err := q.SelectContext(ctx, &rows, query, args...)
	return rows, err
}

func (t table) orderBy(alias string) string {
	if _, ok := t.byName["CREATED_AT"]; ok {
		return fmt.Sprintf(" ORDER BY %sCREATED_AT, %s%s", alias, alias, t.id)
	}
	return fmt.Sprintf(" ORDER BY %s%s", alias, t.id)
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
