package orchestrator

import (
	"context"
	"errors"
	"testing"

	"runvoy/internal/api"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
)

func TestGetImage_Success(t *testing.T) {
	runner := &mockRunner{
		getImageFunc: func(_ context.Context, image string) (*api.ImageInfo, error) {
			return &api.ImageInfo{
				Image: image,
				CPU:   256,
			}, nil
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	imageInfo, err := service.GetImage(context.Background(), "alpine:latest")

	assert.NoError(t, err)
	assert.NotNil(t, imageInfo)
	assert.Equal(t, "alpine:latest", imageInfo.Image)
	assert.Equal(t, 256, imageInfo.CPU)
}

func TestGetImage_NotFound(t *testing.T) {
	runner := &mockRunner{
		getImageFunc: func(_ context.Context, _ string) (*api.ImageInfo, error) {
			return nil, nil
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.GetImage(context.Background(), "nonexistent:latest")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image not found")
}

func TestGetImage_EmptyImageName(t *testing.T) {
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.GetImage(context.Background(), "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image is required")
}

func TestGetImage_RunnerError(t *testing.T) {
	runner := &mockRunner{
		getImageFunc: func(_ context.Context, _ string) (*api.ImageInfo, error) {
			return nil, appErrors.ErrInternalError("test error", errors.New("runner error"))
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.GetImage(context.Background(), "alpine:latest")

	assert.Error(t, err)
}

func TestGetImage_RunnerGenericError(t *testing.T) {
	runner := &mockRunner{
		getImageFunc: func(_ context.Context, _ string) (*api.ImageInfo, error) {
			return nil, errors.New("generic runner error")
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.GetImage(context.Background(), "alpine:latest")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get image")
}

func TestRemoveImage_Success(t *testing.T) {
	runner := &mockRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return nil
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	err := service.RemoveImage(context.Background(), "alpine:latest")

	assert.NoError(t, err)
}

func TestRemoveImage_EmptyImageName(t *testing.T) {
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		"",
		nil,
		nil,
		nil,
	)

	err := service.RemoveImage(context.Background(), "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image is required")
}

func TestRemoveImage_RunnerError(t *testing.T) {
	runner := &mockRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return appErrors.ErrNotFound("image not found", nil)
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	err := service.RemoveImage(context.Background(), "nonexistent:latest")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image not found")
}

func TestRemoveImage_RunnerGenericError(t *testing.T) {
	runner := &mockRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return errors.New("generic runner error")
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	err := service.RemoveImage(context.Background(), "alpine:latest")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove image")
}

func TestListImages_Success(t *testing.T) {
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			return []api.ImageInfo{
				{Image: "alpine:latest", CPU: 256},
				{Image: "ubuntu:22.04", CPU: 512},
			}, nil
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	resp, err := service.ListImages(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Images, 2)
	assert.Equal(t, "alpine:latest", resp.Images[0].Image)
	assert.Equal(t, "ubuntu:22.04", resp.Images[1].Image)
}

func TestListImages_Empty(t *testing.T) {
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			return []api.ImageInfo{}, nil
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	resp, err := service.ListImages(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Images, 0)
}

func TestListImages_RunnerError(t *testing.T) {
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			return nil, appErrors.ErrInternalError("test error", errors.New("runner error"))
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.ListImages(context.Background())

	assert.Error(t, err)
}

func TestListImages_RunnerGenericError(t *testing.T) {
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			return nil, errors.New("generic runner error")
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.ListImages(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list images")
}

func TestRegisterImage_Success(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(_ context.Context, _ string, _ *bool, _ *string, _ *string, _ *int, _ *int, _ *string) error {
			return nil
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	resp, err := service.RegisterImage(context.Background(), "alpine:latest", nil, nil, nil, nil, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "alpine:latest", resp.Image)
	assert.Equal(t, "Image registered successfully", resp.Message)
}

func TestRegisterImage_EmptyImageName(t *testing.T) {
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.RegisterImage(context.Background(), "", nil, nil, nil, nil, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image is required")
}

func TestRegisterImage_RunnerError(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(_ context.Context, _ string, _ *bool, _ *string, _ *string, _ *int, _ *int, _ *string) error {
			return appErrors.ErrBadRequest("invalid image format", nil)
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.RegisterImage(context.Background(), "invalid:image", nil, nil, nil, nil, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image format")
}

func TestRegisterImage_RunnerGenericError(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(_ context.Context, _ string, _ *bool, _ *string, _ *string, _ *int, _ *int, _ *string) error {
			return errors.New("generic runner error")
		},
	}
	logger := testutil.SilentLogger()

	service := NewService(
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		runner,
		logger,
		"",
		nil,
		nil,
		nil,
	)

	_, err := service.RegisterImage(context.Background(), "alpine:latest", nil, nil, nil, nil, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to register image")
}
