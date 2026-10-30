package config

import (
	"os"
	"strings"
)

const (
	environmentKey    = "ENVIRONMENT"
	environmentDev    = "dev"
	environmentReview = "review"
	environmentTest   = "test"
	environmentProd   = "prod"

	sqliteFileKey     = "SQLITE_FILE"
	sqliteFileDefault = "psap.db"
)

func NewEnvVars() IService {
	env := strings.ToLower(os.Getenv(environmentKey))
	if env == "" {
		env = environmentDev
	}

	p := envVarsService{}
	p.populateFromEnvVars()

	return &p
}

type envVarsService struct {
	sqliteFile string
}

func (svc *envVarsService) populateFromEnvVars() {
	svc.sqliteFile = os.Getenv(sqliteFileKey)
	if svc.sqliteFile == "" {
		svc.sqliteFile = sqliteFileDefault
	}
}

func (svc *envVarsService) SQLiteFile() string {
	return svc.sqliteFile
}

func read(file string) []byte {
	fd, err := os.ReadFile(file)
	if err != nil {
		return []byte{}
	}

	return fd

}
