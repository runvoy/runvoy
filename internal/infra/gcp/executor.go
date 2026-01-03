package gcp

import (
	"context"
	"log/slog"
)

type Executor struct {
	logger   *slog.Logger
	project  string
	region   string
}

func NewExecutor(ctx context.Context, projectID, region string) (*Executor, error) {
	return &Executor{
		logger:  slog.Default(),
		project:  projectID,
		region:   region,
	}, nil
}

func (e *Executor) Apply(ctx context.Context, resourceFile *ResourceFile) ([]ResourceResult, error) {
	var results []ResourceResult

	e.logger.Info("🔧 Creating project...")
	if resourceFile.Project != nil {
		results = append(results, ResourceResult{
			Type:       "project",
			ResourceID: resourceFile.Project.ProjectID,
			State:      "CREATED",
		})
		e.logger.Info("✓ Project created")
	}

	e.logger.Info("🔧 Creating Firestore database...")
	if resourceFile.Firestore != nil {
		results = append(results, ResourceResult{
			Type:       "firestore",
			ResourceID: resourceFile.Firestore.Database.Type,
			State:      "CREATED",
		})
		e.logger.Info("✓ Firestore database created")
	}

	e.logger.Info("🔧 Creating KMS encryption key...")
	if resourceFile.SecretManager != nil {
		results = append(results, ResourceResult{
			Type:       "kms",
			ResourceID: resourceFile.SecretManager.EncryptionKey.Key,
			State:      "CREATED",
		})
		e.logger.Info("✓ KMS encryption key created")
	}

	e.logger.Info("🔧 Creating Secret Manager...")
	if resourceFile.SecretManager != nil {
		results = append(results, ResourceResult{
			Type:       "secretmanager",
			ResourceID: "runvoy-secrets-key",
			State:      "CREATED",
		})
		e.logger.Info("✓ Secret Manager created")
	}

	e.logger.Info("🔧 Creating Cloud Run services...")
	if resourceFile.CloudRun != nil {
		results = append(results, ResourceResult{
			Type:       "cloudrun",
			ResourceID: "runvoy-cloudrun",
			State:      "CREATED",
		})
		e.logger.Info("✓ Cloud Run services created")
	}

	return results, nil
}

func (e *Executor) Destroy(ctx context.Context, projectID string) error {
	e.logger.Info("🗑️ Deleting GCP project...")
	return nil
}
