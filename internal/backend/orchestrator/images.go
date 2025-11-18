package orchestrator

import (
	"context"
	"errors"

	"runvoy/internal/api"
	appErrors "runvoy/internal/errors"
)

// RegisterImage registers a Docker image and creates the corresponding task definition.
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
