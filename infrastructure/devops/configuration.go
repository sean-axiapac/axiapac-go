package devops

import (
	"context"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type DBEntry struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type DBConfig struct {
	Databases []DBEntry `yaml:"databases"`
}

var (
	once    sync.Once
	dbList  []DBEntry
	loadErr error
)

func LoadDBConfig(ctx context.Context) ([]DBEntry, error) {
	once.Do(func() {
		paramName := "databases"

		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			loadErr = fmt.Errorf("load aws config: %w", err)
			return
		}

		client := ssm.NewFromConfig(cfg)

		out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
			Name:           aws.String(paramName),
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			loadErr = fmt.Errorf("get parameter: %w", err)
			return
		}

		var parsed []DBEntry
		if err := yaml.Unmarshal([]byte(*out.Parameter.Value), &parsed); err != nil {
			loadErr = fmt.Errorf("unmarshal yaml: %w", err)
			return
		}

		dbList = parsed
	})

	return dbList, loadErr
}
