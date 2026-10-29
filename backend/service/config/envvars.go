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
}

func (svc *envVarsService) populateFromEnvVars() {
}

func read(file string) []byte {
	fd, err := os.ReadFile(file)
	if err != nil {
		return []byte{}
	}

	return fd

}
