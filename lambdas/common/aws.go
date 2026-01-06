package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"gopkg.in/yaml.v3"
)

type DBEntry struct {
	Name     string `yaml:"name" json:"name"`
	Host     string `yaml:"host" json:"host"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

func (db DBEntry) GetDSN(dbname string) string {
	// username:password@tcp(host:3306)/name?parseTime=true
	// Assuming host might already contain port or default to 3306
	host := db.Host
	if !strings.Contains(host, ":") {
		host = host + ":3306"
	}
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true", db.Username, db.Password, host, dbname)
}

func LoadDatabases(ctx context.Context) (map[string]DBEntry, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := ssm.NewFromConfig(cfg)
	paramName := "databases"

	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get parameter %s: %w", paramName, err)
	}

	if out.Parameter == nil || out.Parameter.Value == nil {
		return nil, fmt.Errorf("parameter %s is empty", paramName)
	}

	// Based on infrastructure/devops/configuration.go, it's a list of entries
	var entries []DBEntry
	if err := yaml.Unmarshal([]byte(*out.Parameter.Value), &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal databases: %w", err)
	}

	result := make(map[string]DBEntry)
	for _, entry := range entries {
		result[strings.ToLower(entry.Name)] = entry
	}

	return result, nil
}
