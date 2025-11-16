package orchestrator

import (
	"context"
	"errors"
	"testing"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
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

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	imageInfo, imageErr := service.GetImage(context.Background(), "alpine:latest")

	assert.NoError(t, imageErr)
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

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, imageErr := service.GetImage(context.Background(), "nonexistent:latest")

	assert.Error(t, imageErr)
	assert.Contains(t, imageErr.Error(), "image not found")
}

func TestGetImage_EmptyImageName(t *testing.T) {
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, imageErr := service.GetImage(context.Background(), "")

	assert.Error(t, imageErr)
	assert.Contains(t, imageErr.Error(), "image is required")
}

func TestGetImage_RunnerError(t *testing.T) {
	runner := &mockRunner{
		getImageFunc: func(_ context.Context, _ string) (*api.ImageInfo, error) {
			return nil, apperrors.ErrInternalError("runner error", nil)
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, imageErr := service.GetImage(context.Background(), "alpine:latest")

	assert.Error(t, imageErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(imageErr, &appErr))
}

func TestGetImage_RunnerGenericError(t *testing.T) {
	runner := &mockRunner{
		getImageFunc: func(_ context.Context, _ string) (*api.ImageInfo, error) {
			return nil, errors.New("some runner error")
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, imageErr := service.GetImage(context.Background(), "alpine:latest")

	assert.Error(t, imageErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(imageErr, &appErr))
}

func TestRemoveImage_Success(t *testing.T) {
	runner := &mockRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	removeErr := service.RemoveImage(context.Background(), "alpine:latest")

	assert.NoError(t, removeErr)
}

func TestRemoveImage_EmptyImageName(t *testing.T) {
	runner := &mockRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	removeErr := service.RemoveImage(context.Background(), "")

	assert.Error(t, removeErr)
	assert.Contains(t, removeErr.Error(), "image is required")
}

func TestRemoveImage_RunnerError(t *testing.T) {
	runner := &mockRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return apperrors.ErrInternalError("runner error", nil)
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	removeErr := service.RemoveImage(context.Background(), "nonexistent:latest")

	assert.Error(t, removeErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(removeErr, &appErr))
}

func TestRemoveImage_RunnerGenericError(t *testing.T) {
	runner := &mockRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return errors.New("some runner error")
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	removeErr := service.RemoveImage(context.Background(), "alpine:latest")

	assert.Error(t, removeErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(removeErr, &appErr))
}

func TestListImages_Success(t *testing.T) {
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			return []api.ImageInfo{
				{Image: "alpine:latest", CPU: 256},
				{Image: "ubuntu:20.04", CPU: 512},
			}, nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, listErr := service.ListImages(context.Background())

	assert.NoError(t, listErr)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Images, 2)
	assert.Equal(t, "alpine:latest", resp.Images[0].Image)
}

func TestListImages_Empty(t *testing.T) {
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			return []api.ImageInfo{}, nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, listErr := service.ListImages(context.Background())

	assert.NoError(t, listErr)
	assert.Len(t, resp.Images, 0)
}

func TestListImages_RunnerError(t *testing.T) {
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			return nil, apperrors.ErrInternalError("runner error", nil)
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, listErr := service.ListImages(context.Background())

	assert.Error(t, listErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(listErr, &appErr))
}

func TestListImages_RunnerGenericError(t *testing.T) {
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			return nil, errors.New("some runner error")
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, listErr := service.ListImages(context.Background())

	assert.Error(t, listErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(listErr, &appErr))
}

func TestRegisterImage_Success(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(_ context.Context, _ string, _ *bool, _ *string, _ *string, _ *int, _ *int, _ *string) error {
			return nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, registerErr := service.RegisterImage(context.Background(), "alpine:latest", nil, nil, nil, nil, nil, nil)

	assert.NoError(t, registerErr)
	assert.NotNil(t, resp)
}

func TestRegisterImage_EmptyImageName(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(_ context.Context, _ string, _ *bool, _ *string, _ *string, _ *int, _ *int, _ *string) error {
			return nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, registerErr := service.RegisterImage(context.Background(), "", nil, nil, nil, nil, nil, nil)

	assert.Error(t, registerErr)
	assert.Contains(t, registerErr.Error(), "image is required")
}

func TestRegisterImage_RunnerError(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(_ context.Context, _ string, _ *bool, _ *string, _ *string, _ *int, _ *int, _ *string) error {
			return apperrors.ErrInternalError("runner error", nil)
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, registerErr := service.RegisterImage(context.Background(), "invalid:image", nil, nil, nil, nil, nil, nil)

	assert.Error(t, registerErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(registerErr, &appErr))
}

func TestRegisterImage_RunnerGenericError(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(_ context.Context, _ string, _ *bool, _ *string, _ *string, _ *int, _ *int, _ *string) error {
			return errors.New("some runner error")
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(
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
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, registerErr := service.RegisterImage(context.Background(), "alpine:latest", nil, nil, nil, nil, nil, nil)

	assert.Error(t, registerErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(registerErr, &appErr))
}
