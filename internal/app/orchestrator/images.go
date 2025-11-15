package orchestrator

import (
	"context"
	"errors"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
)

// RegisterImage registers a Docker image and creates the corresponding task definition.
func (s *Service) RegisterImage(
	ctx context.Context,
	image string,
	isDefault *bool,
	taskRoleName *string,
	taskExecutionRoleName *string,
	cpu *int,
	memory *int,
	runtimePlatform *string,
) (*api.RegisterImageResponse, error) {
	if image == "" {
		return nil, apperrors.ErrBadRequest("image is required", nil)
	}

	if err := s.runner.RegisterImage(
		ctx, image, isDefault, taskRoleName, taskExecutionRoleName, cpu, memory, runtimePlatform,
	); err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, apperrors.ErrInternalError("failed to register image", err)
	}

	return &api.RegisterImageResponse{
		Image:   image,
		Message: "Image registered successfully",
	}, nil
}

// ListImages returns all registered Docker images.
func (s *Service) ListImages(ctx context.Context) (*api.ListImagesResponse, error) {
	images, err := s.runner.ListImages(ctx)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, apperrors.ErrInternalError("failed to list images", err)
	}

	return &api.ListImagesResponse{
		Images: images,
	}, nil
}

// GetImage returns a single registered Docker image by ID or name.
func (s *Service) GetImage(ctx context.Context, image string) (*api.ImageInfo, error) {
	if image == "" {
		return nil, apperrors.ErrBadRequest("image is required", nil)
	}

	imageInfo, err := s.runner.GetImage(ctx, image)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, apperrors.ErrInternalError("failed to get image", err)
	}

	if imageInfo == nil {
		return nil, apperrors.ErrNotFound("image not found", nil)
	}

	return imageInfo, nil
}

// RemoveImage removes a Docker image and deregisters its task definitions.
func (s *Service) RemoveImage(ctx context.Context, image string) error {
	if image == "" {
		return apperrors.ErrBadRequest("image is required", nil)
	}

	if err := s.runner.RemoveImage(ctx, image); err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return err
		}
		return apperrors.ErrInternalError("failed to remove image", err)
	}

	return nil
}
