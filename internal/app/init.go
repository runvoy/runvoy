package app

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"runvoy/internal/database"
	dynamorepo "runvoy/internal/database/dynamodb"
)

// InitConfig holds configuration options for service initialization
type InitConfig struct {
	// EnableDynamoDB controls whether to attempt DynamoDB initialization
	EnableDynamoDB bool

	// APIKeysTableName is the DynamoDB table name (if empty, reads from env)
	APIKeysTableName string

	// AWSConfig allows passing a pre-configured AWS config (optional)
	AWSConfig *aws.Config

	// Logger for initialization messages (if nil, uses log package)
	Logger Logger
}

// Logger interface for initialization logging
type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

// defaultLogger wraps the standard log package
type defaultLogger struct{}

func (l *defaultLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *defaultLogger) Println(v ...interface{}) {
	log.Println(v...)
}

// Initialize creates a new Service with all dependencies configured based on environment
// and the provided configuration. This function handles:
// - Loading AWS configuration
// - Creating DynamoDB client
// - Setting up UserRepository
// - Creating the Service instance
func Initialize(ctx context.Context, cfg *InitConfig) (*Service, error) {
	if cfg == nil {
		cfg = &InitConfig{}
	}

	// Use default logger if none provided
	logger := cfg.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	var userRepo database.UserRepository

	// Determine if we should initialize DynamoDB
	shouldInitDB := cfg.EnableDynamoDB || cfg.APIKeysTableName != "" || os.Getenv("API_KEYS_TABLE") != ""

	if shouldInitDB {
		var err error
		userRepo, err = initializeDynamoDB(ctx, cfg, logger)
		if err != nil {
			// Non-fatal: Service can still operate without user management
			logger.Printf("WARNING: Could not initialize DynamoDB: %v", err)
			logger.Println("→ Service will run without user management features")
		} else if userRepo != nil {
			logger.Println("→ User repository initialized successfully")
		}
	} else {
		logger.Println("→ DynamoDB not configured, user management disabled")
	}

	return NewService(userRepo), nil
}

// initializeDynamoDB handles DynamoDB-specific initialization
func initializeDynamoDB(ctx context.Context, cfg *InitConfig, logger Logger) (database.UserRepository, error) {
	// Get table name
	tableName := cfg.APIKeysTableName
	if tableName == "" {
		tableName = os.Getenv("API_KEYS_TABLE")
	}

	if tableName == "" {
		return nil, fmt.Errorf("API_KEYS_TABLE not set")
	}

	// Load or use provided AWS config
	var awsCfg aws.Config
	var err error

	if cfg.AWSConfig != nil {
		awsCfg = *cfg.AWSConfig
	} else {
		awsCfg, err = config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
	}

	// Create DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	// Create and return repository
	return dynamorepo.NewUserRepository(dynamoClient, tableName), nil
}

// MustInitialize is like Initialize but panics on error
// Useful for Lambda functions where we want to fail fast during cold start
func MustInitialize(ctx context.Context, cfg *InitConfig) *Service {
	svc, err := Initialize(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize service: %v", err)
	}
	return svc
}
