package authorization

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"runvoy/internal/api"
	"runvoy/internal/database"
)

// ImageRepository defines the interface for listing images.
// This is a minimal interface to avoid import cycles.
type ImageRepository interface {
	ListImages(ctx context.Context) ([]api.ImageInfo, error)
}

// HydrateEnforcer loads all user roles and resource ownerships into the Casbin enforcer.
// This should be called during initialization to populate the enforcer with current data.
func HydrateEnforcer(
	ctx context.Context,
	enforcer *Enforcer,
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	secretsRepo database.SecretsRepository,
	imageRepo ImageRepository,
	logger *slog.Logger,
) error {
	if err := loadUserRoles(ctx, enforcer, userRepo, logger); err != nil {
		return fmt.Errorf("failed to load user roles: %w", err)
	}

	if err := loadResourceOwnerships(ctx, enforcer, executionRepo, secretsRepo, imageRepo, logger); err != nil {
		return fmt.Errorf("failed to load resource ownerships: %w", err)
	}

	logger.Debug("casbin authorization enforcer hydrated successfully")
	return nil
}

func loadUserRoles(
	ctx context.Context,
	enforcer *Enforcer,
	userRepo database.UserRepository,
	_ *slog.Logger,
) error {
	users, err := userRepo.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to load users: %w", err)
	}

	g, _ := errgroup.WithContext(ctx)

	for _, user := range users {
		g.Go(func() error {
			if user == nil || user.Email == "" {
				return errors.New("user is nil or missing email")
			}

			role, roleErr := NewRole(user.Role)
			if roleErr != nil {
				return fmt.Errorf("user %s has invalid role %q: %w", user.Email, user.Role, roleErr)
			}

			if addErr := enforcer.AddRoleForUser(user.Email, role); addErr != nil {
				return fmt.Errorf("failed to add role %q for user %s: %w", user.Role, user.Email, addErr)
			}

			return nil
		})
	}

	if waitErr := g.Wait(); waitErr != nil {
		return fmt.Errorf("failed to load user roles: %w", waitErr)
	}

	return nil
}

func loadResourceOwnerships(
	ctx context.Context,
	enforcer *Enforcer,
	executionRepo database.ExecutionRepository,
	secretsRepo database.SecretsRepository,
	imageRepo ImageRepository,
	_ *slog.Logger,
) error {
	if err := loadSecretOwnerships(ctx, enforcer, secretsRepo); err != nil {
		return fmt.Errorf("failed to load secret ownerships: %w", err)
	}

	if err := loadExecutionOwnerships(ctx, enforcer, executionRepo); err != nil {
		return fmt.Errorf("failed to load execution ownerships: %w", err)
	}

	if imageRepo != nil {
		if err := loadImageOwnerships(ctx, enforcer, imageRepo); err != nil {
			return fmt.Errorf("failed to load image ownerships: %w", err)
		}
	}

	return nil
}

func loadSecretOwnerships(
	ctx context.Context,
	enforcer *Enforcer,
	secretsRepo database.SecretsRepository,
) error {
	secrets, err := secretsRepo.ListSecrets(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to load secrets: %w", err)
	}

	for _, secret := range secrets {
		if secret == nil || secret.Name == "" || secret.CreatedBy == "" {
			return errors.New("secret is nil or missing required fields")
		}

		resourceID := FormatResourceID("secret", secret.Name)
		for _, owner := range secret.OwnedBy {
			if addErr := enforcer.AddOwnershipForResource(resourceID, owner); addErr != nil {
				return fmt.Errorf("failed to add ownership for secret %s: %w", secret.Name, addErr)
			}
		}
	}

	return nil
}

func loadExecutionOwnerships(
	ctx context.Context,
	enforcer *Enforcer,
	executionRepo database.ExecutionRepository,
) error {
	executions, err := executionRepo.ListExecutions(ctx, 0, nil)
	if err != nil {
		return fmt.Errorf("failed to load executions: %w", err)
	}

	g, _ := errgroup.WithContext(ctx)

	for _, execution := range executions {
		g.Go(func() error {
			if execution == nil || execution.ExecutionID == "" || execution.CreatedBy == "" {
				return errors.New("execution is nil or missing required fields")
			}

			resourceID := FormatResourceID("execution", execution.ExecutionID)
			for _, owner := range execution.OwnedBy {
				if addErr := enforcer.AddOwnershipForResource(resourceID, owner); addErr != nil {
					return fmt.Errorf("failed to add ownership for execution %s: %w", execution.ExecutionID, addErr)
				}
			}
			return nil
		})
	}

	if waitErr := g.Wait(); waitErr != nil {
		return fmt.Errorf("failed to load execution ownerships: %w", waitErr)
	}

	return nil
}

func loadImageOwnerships(
	ctx context.Context,
	enforcer *Enforcer,
	imageRepo ImageRepository,
) error {
	images, err := imageRepo.ListImages(ctx)
	if err != nil {
		return fmt.Errorf("failed to load images: %w", err)
	}

	g, _ := errgroup.WithContext(ctx)

	for i := range images {
		g.Go(func() error {
			if images[i].ImageID == "" || images[i].CreatedBy == "" {
				return errors.New("image is missing required fields")
			}

			resourceID := FormatResourceID("image", images[i].ImageID)
			for _, owner := range images[i].OwnedBy {
				if addErr := enforcer.AddOwnershipForResource(resourceID, owner); addErr != nil {
					return fmt.Errorf("failed to add ownership for image %s: %w", images[i].ImageID, addErr)
				}
			}
			return nil
		})
	}

	if waitErr := g.Wait(); waitErr != nil {
		return fmt.Errorf("failed to load image ownerships: %w", waitErr)
	}

	return nil
}
