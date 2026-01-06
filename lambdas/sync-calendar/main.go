package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"encoding/json"

	"axiapac.com/axiapac/core"
	"axiapac.com/axiapac/lambdas/common"
	"github.com/aws/aws-lambda-go/lambda"
	"gorm.io/gorm"
)

type SyncEvent struct {
	Databases *[]string `json:"databases"`
	DryRun    bool      `json:"dryRun"`
	Env       string    `json:"env"`
}

func SyncCalendar(ctx context.Context, dsn string, databases *[]string, dryRun bool) (map[string]SyncStats, error) {
	holidays, err := GetPublicHolidays(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get public holidays: %w", err)
	}

	fmt.Printf("[INFO] Successfully processed %d regions from S3\n", len(holidays))

	dm, err := core.New(dsn, 10)
	dm.LogLevel = core.LogLevelError
	if err != nil {
		return nil, fmt.Errorf("failed to create database manager: %w", err)
	}
	defer dm.Close()

	var targetDatabases []string
	if databases == nil {
		fmt.Printf("[INFO] No databases provided, fetching all databases...\n")
		targetDatabases, err = dm.GetAllDatabases(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get all databases: %w", err)
		}
	} else {
		targetDatabases = *databases
	}

	results := make(map[string]SyncStats)
	for _, dbName := range targetDatabases {
		fmt.Printf("[INFO] Syncing for database: %s\n", dbName)
		// dm.Exec internally calls USE `schema` to switch the database context safely for GORM
		err := dm.Exec(ctx, dbName, func(db *gorm.DB) error {
			stats, err := SyncHolidays(db, holidays, dryRun)
			if err != nil {
				return err
			}
			results[dbName] = stats
			return nil
		})
		if err != nil {
			fmt.Printf("[ERROR] failed to sync for database %s: %v\n", dbName, err)
			continue
		}
	}

	fmt.Printf("[INFO] Finished syncing holidays to database(s)\n")
	return results, nil
}

func HandleRequest(ctx context.Context, event interface{}) (interface{}, error) {
	eventJson, _ := json.Marshal(event)
	fmt.Printf("[INFO] Event: %s\n", string(eventJson))

	var syncEvent SyncEvent
	var bedrockEvent common.BedrockEvent

	// First, try to see if it's a Bedrock event
	_ = json.Unmarshal(eventJson, &bedrockEvent)

	if bedrockEvent.ActionGroup != "" {
		fmt.Printf("[INFO] Identified as Bedrock Event: %s\n", bedrockEvent.ActionGroup)
		// Extract parameters from Bedrock format
		if dbParam := bedrockEvent.GetParameter("databases"); dbParam != "" {
			parts := strings.Split(dbParam, ",")
			var dbs []string
			for _, d := range parts {
				if trimmed := strings.TrimSpace(d); trimmed != "" {
					dbs = append(dbs, trimmed)
				}
			}
			if len(dbs) > 0 {
				syncEvent.Databases = &dbs
			}
		}
		if dryRunParam := bedrockEvent.GetParameter("dryrun"); strings.ToLower(dryRunParam) == "true" {
			syncEvent.DryRun = true
		}
		if envParam := bedrockEvent.GetParameter("environment"); envParam != "" {
			syncEvent.Env = envParam
		} else if envParam := bedrockEvent.GetParameter("env"); envParam != "" {
			syncEvent.Env = envParam
		}
	} else {
		fmt.Printf("[INFO] Identified as Standard Sync Event\n")
		if err := json.Unmarshal(eventJson, &syncEvent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sync event: %w", err)
		}
	}

	syncEventJson, _ := json.Marshal(syncEvent)
	fmt.Printf("[INFO] SyncEvent: %s\n", string(syncEventJson))

	// Load DSN from SSM
	fmt.Printf("[INFO] Loading database configuration from SSM parameter store 'databases'\n")
	dbs, err := common.LoadDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load databases from SSM: %w", err)
	}

	env := strings.ToLower(syncEvent.Env)
	if env == "" {
		return nil, fmt.Errorf("environment (env) is required")
	}

	entry, ok := dbs[env]
	if !ok {
		return nil, fmt.Errorf("environment '%s' not found in parameter store", env)
	}
	dsn := entry.GetDSN("")
	fmt.Printf("[INFO] Using DSN for environment: %s\n", env)

	results, err := SyncCalendar(ctx, dsn, syncEvent.Databases, syncEvent.DryRun)
	if err != nil {
		return nil, err
	}

	// If it's a Bedrock request
	if bedrockEvent.ActionGroup != "" && bedrockEvent.Function != "" {
		return common.NewBedrockResponse(bedrockEvent.ActionGroup, bedrockEvent.Function, results), nil
	}

	return results, nil
}

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(HandleRequest)
	} else {
		dbs, err := common.LoadDatabases(context.Background())
		if err != nil {
			fmt.Printf("[ERROR] %v\n", err)
			os.Exit(1)
		}
		dsn := dbs["dev"].GetDSN("")
		fmt.Printf("[INFO] DSN: %s\n", dsn)

		dryRun := true
		results, err := SyncCalendar(context.Background(),
			dsn,
			nil, //utils.Ptr([]string{"axiapacmvc"}),
			dryRun)
		if err != nil {
			fmt.Printf("[ERROR] %v\n", err)
			os.Exit(1)
		}
		resJson, _ := json.MarshalIndent(results, "", "  ")
		fmt.Printf("[SUCCESS] Results:\n%s\n", string(resJson))
	}
}
