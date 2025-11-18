package orchestrator

import (
	"context"
	"errors"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	appErrors "runvoy/internal/errors"
)

// RegisterImage registers a Docker image and creates the corresponding task definition.
// After successful registration, ownership is synced to the Casbin enforcer to maintain
// consistency with the database. If ownership sync fails, an error is logged but
// registration succeeds; ownership will be synced during the next hydration cycle or
// health reconcile.
func (s *Service) RegisterImage(
	ctx context.Context,
	req *api.RegisterImageRequest,
	createdBy string,
) (*api.RegisterImageResponse, error) {
	if req == nil {
		return nil, appErrors.ErrBadRequest("request is required", nil)
	}

	if req.Image == "" {
		return nil, appErrors.ErrBadRequest("image is required", nil)
	}

	if createdBy == "" {
		return nil, appErrors.ErrBadRequest("createdBy is required", nil)
	}

	if err := s.runner.RegisterImage(
		ctx,
		req.Image,
		req.IsDefault,
		req.TaskRoleName,
		req.TaskExecutionRoleName,
		req.CPU,
		req.Memory,
		req.RuntimePlatform,
		createdBy,
	); err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, appErrors.ErrInternalError("failed to register image", err)
	}

	imageInfo, getErr := s.runner.GetImage(ctx, req.Image)
	if getErr != nil {
		s.Logger.Error("failed to get image info after registration for ownership sync",
			"image", req.Image,
			"error", getErr,
		)
	} else if imageInfo != nil && imageInfo.ImageID != "" {
		resourceID := authorization.FormatResourceID("image", imageInfo.ImageID)
		for _, owner := range imageInfo.OwnedBy {
			if syncErr := s.enforcer.AddOwnershipForResource(resourceID, owner); syncErr != nil {
				s.Logger.Error("failed to sync image ownership to enforcer after registration",
					"image_id", imageInfo.ImageID,
					"owner", owner,
					"error", syncErr,
				)
			}
		}
	}

	return &api.RegisterImageResponse{
		Image:   req.Image,
		Message: "Image registered successfully",
	}, nil
}

// ListImages returns all registered Docker images.
func (s *Service) ListImages(ctx context.Context) (*api.ListImagesResponse, error) {
	images, err := s.runner.ListImages(ctx)
	if err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, appErrors.ErrInternalError("failed to list images", err)
	}

	return &api.ListImagesResponse{
		Images: images,
	}, nil
}

// GetImage returns a single registered Docker image by ID or name.
func (s *Service) GetImage(ctx context.Context, image string) (*api.ImageInfo, error) {
	if image == "" {
		return nil, appErrors.ErrBadRequest("image is required", nil)
	}

	imageInfo, err := s.runner.GetImage(ctx, image)
	if err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, appErrors.ErrInternalError("failed to get image", err)
	}

	if imageInfo == nil {
		return nil, appErrors.ErrNotFound("image not found", nil)
	}

	return imageInfo, nil
}

// RemoveImage removes a Docker image and deregisters its task definitions.
func (s *Service) RemoveImage(ctx context.Context, image string) error {
	if image == "" {
		return appErrors.ErrBadRequest("image is required", nil)
	}

	if err := s.runner.RemoveImage(ctx, image); err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			return err
		}
		return appErrors.ErrInternalError("failed to remove image", err)
	}

	return nil
}

// ResolveImage resolves a user-provided image string to a specific ImageInfo.
// If image string is empty, returns the default image.
// This centralizes image resolution logic for authorization and execution.
func (s *Service) ResolveImage(ctx context.Context, image string) (*api.ImageInfo, error) {
	// If no image specified, use default
	if image == "" {
		imageInfo, err := s.runner.GetImage(ctx, "")
		if err != nil {
			var appErr *appErrors.AppError
			if errors.As(err, &appErr) {
				return nil, err
			}
			return nil, appErrors.ErrInternalError("failed to get default image", err)
		}
		if imageInfo == nil {
			return nil, appErrors.ErrBadRequest("no image specified and no default image configured", nil)
		}
		return imageInfo, nil
	}

	// Resolve the provided image string
	imageInfo, err := s.runner.GetImage(ctx, image)
	if err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, appErrors.ErrInternalError("failed to resolve image", err)
	}

	if imageInfo == nil {
		return nil, appErrors.ErrBadRequest("image not registered", nil)
	}

	return imageInfo, nil
}
