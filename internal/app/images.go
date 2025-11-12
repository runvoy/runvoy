package app

import (
	"context"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
)

// RegisterImage registers a Docker image and creates the corresponding task definition.
func (s *Service) RegisterImage(
	ctx context.Context,
	image string,
	isDefault *bool,
) (*api.RegisterImageResponse, error) {
	if image == "" {
		return nil, apperrors.ErrBadRequest("image is required", nil)
	}

	if err := s.runner.RegisterImage(ctx, image, isDefault); err != nil {
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
		return nil, apperrors.ErrInternalError("failed to list images", err)
	}

	return &api.ListImagesResponse{
		Images: images,
	}, nil
}

// RemoveImage removes a Docker image and deregisters its task definitions.
func (s *Service) RemoveImage(ctx context.Context, image string) error {
	if image == "" {
		return apperrors.ErrBadRequest("image is required", nil)
	}

	if err := s.runner.RemoveImage(ctx, image); err != nil {
		return apperrors.ErrInternalError("failed to remove image", err)
	}

	return nil
}
