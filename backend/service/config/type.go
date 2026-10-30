package config

type IService interface {
	// SQLiteFile is the path of the SQLite database file used by the data service.
	SQLiteFile() string
}
