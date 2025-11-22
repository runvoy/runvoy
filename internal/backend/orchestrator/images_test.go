package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/backend/health"
	"runvoy/internal/backend/orchestrator/interfaces"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
)

// newTestEnforcer creates a test enforcer for image tests
func newTestEnforcer(t *testing.T) *authorization.Enforcer {
	enf, err := authorization.NewEnforcer(testutil.SilentLogger())
	if err != nil {
		t.Fatal(err)
	}
	return enf
}

// newImageTestService creates a Service for image testing with a custom runner.
// The runner parameter implements all 4 interfaces (TaskManager, ImageRegistry, LogManager, ObservabilityManager).
func newImageTestService(t *testing.T, runner *mockRunner) *Service {
	taskManager := interfaces.TaskManager(runner)
	imageRegistry := interfaces.ImageRegistry(runner)
	logManager := interfaces.LogManager(runner)
	observabilityManager := interfaces.ObservabilityManager(runner)

	svc, err := NewService(
		context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockImageRepository{},
		taskManager,
		imageRegistry,
		logManager,
		observabilityManager,
		testutil.SilentLogger(),
		constants.AWS,
		&mockWebSocketManager{},
		&mockSecretsRepository{},
		&mockHealthManager{},
		newTestEnforcer(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// mockHealthManager implements health.Manager for testing
type mockHealthManager struct{}

func (m *mockHealthManager) Reconcile(_ context.Context) (*health.Report, error) {
	return &health.Report{}, nil
}

func TestGetImage_Success(t *testing.T) {
	runner := &mockRunner{
		getImageFunc: func(_ context.Context, image string) (*api.ImageInfo, error) {
			return &api.ImageInfo{
				Image: image,
				CPU:   256,
			}, nil
		},
	}
	service := newImageTestService(t, runner)

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
	service := newImageTestService(t, runner)

	_, imageErr := service.GetImage(context.Background(), "nonexistent:latest")

	assert.Error(t, imageErr)
	assert.Contains(t, imageErr.Error(), "image not found")
}

func TestGetImage_EmptyImageName(t *testing.T) {
	runner := &mockRunner{}
	service := newImageTestService(t, runner)

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
	service := newImageTestService(t, runner)

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
	service := newImageTestService(t, runner)

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
	service := newImageTestService(t, runner)

	removeErr := service.RemoveImage(context.Background(), "alpine:latest")

	assert.NoError(t, removeErr)
}

func TestRemoveImage_EmptyImageName(t *testing.T) {
	runner := &mockRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return nil
		},
	}
	service := newImageTestService(t, runner)

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
	service := newImageTestService(t, runner)

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
	service := newImageTestService(t, runner)

	removeErr := service.RemoveImage(context.Background(), "alpine:latest")

	assert.Error(t, removeErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(removeErr, &appErr))
}

func TestListImages_Success(t *testing.T) {
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			return []api.ImageInfo{
				{Image: "alpine:latest", ImageID: "alpine:latest", CreatedBy: "test@example.com",
					OwnedBy: []string{"user@example.com"}},
				{Image: "ubuntu:20.04", ImageID: "ubuntu:20.04", CreatedBy: "test@example.com",
					OwnedBy: []string{"user@example.com"}},
			}, nil
		},
	}
	service := newImageTestService(t, runner)

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
	service := newImageTestService(t, runner)

	resp, listErr := service.ListImages(context.Background())

	assert.NoError(t, listErr)
	assert.Len(t, resp.Images, 0)
}

func TestListImages_RunnerError(t *testing.T) {
	callCount := 0
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			callCount++
			// Return empty list during initialization, error on test call
			if callCount == 1 {
				return []api.ImageInfo{}, nil
			}
			return nil, apperrors.ErrInternalError("runner error", nil)
		},
	}
	service := newImageTestService(t, runner)

	_, listErr := service.ListImages(context.Background())

	assert.Error(t, listErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(listErr, &appErr))
}

func TestListImages_RunnerGenericError(t *testing.T) {
	callCount := 0
	runner := &mockRunner{
		listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
			callCount++
			// Return empty list during initialization, error on test call
			if callCount == 1 {
				return []api.ImageInfo{}, nil
			}
			return nil, errors.New("some runner error")
		},
	}
	service := newImageTestService(t, runner)

	_, listErr := service.ListImages(context.Background())

	assert.Error(t, listErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(listErr, &appErr))
}

func TestRegisterImage_Success(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(
			_ context.Context, _ string, _ *bool, _ *string, _ *string,
			_ *int, _ *int, _ *string, _ string,
		) error {
			return nil
		},
	}
	service := newImageTestService(t, runner)

	resp, registerErr := service.RegisterImage(
		context.Background(),
		&api.RegisterImageRequest{Image: "alpine:latest"},
		"test@example.com",
	)

	assert.NoError(t, registerErr)
	assert.NotNil(t, resp)
}

func TestRegisterImage_EmptyImageName(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(
			_ context.Context, _ string, _ *bool, _ *string, _ *string,
			_ *int, _ *int, _ *string, _ string,
		) error {
			return nil
		},
	}
	service := newImageTestService(t, runner)

	_, registerErr := service.RegisterImage(
		context.Background(),
		&api.RegisterImageRequest{Image: ""},
		"test@example.com",
	)

	assert.Error(t, registerErr)
	assert.Contains(t, registerErr.Error(), "image is required")
}

func TestRegisterImage_RunnerError(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(
			_ context.Context, _ string, _ *bool, _ *string, _ *string,
			_ *int, _ *int, _ *string, _ string,
		) error {
			return apperrors.ErrInternalError("runner error", nil)
		},
	}
	service := newImageTestService(t, runner)

	_, registerErr := service.RegisterImage(
		context.Background(),
		&api.RegisterImageRequest{Image: "invalid:image"},
		"test@example.com",
	)

	assert.Error(t, registerErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(registerErr, &appErr))
}

func TestRegisterImage_RunnerGenericError(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(
			_ context.Context, _ string, _ *bool, _ *string, _ *string,
			_ *int, _ *int, _ *string, _ string,
		) error {
			return errors.New("some runner error")
		},
	}
	service := newImageTestService(t, runner)

	_, registerErr := service.RegisterImage(
		context.Background(),
		&api.RegisterImageRequest{Image: "alpine:latest"},
		"test@example.com",
	)

	assert.Error(t, registerErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(registerErr, &appErr))
}

func TestRegisterImage_EmptyCreatedBy(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(
			_ context.Context, _ string, _ *bool, _ *string, _ *string,
			_ *int, _ *int, _ *string, _ string,
		) error {
			return nil
		},
	}
	service := newImageTestService(t, runner)

	_, registerErr := service.RegisterImage(
		context.Background(),
		&api.RegisterImageRequest{Image: "alpine:latest"},
		"",
	)

	assert.Error(t, registerErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(registerErr, &appErr))
	assert.Contains(t, registerErr.Error(), "createdBy is required")
	assert.Equal(t, apperrors.ErrCodeInvalidRequest, apperrors.GetErrorCode(registerErr))
	assert.Equal(t, http.StatusBadRequest, apperrors.GetStatusCode(registerErr))
}

func TestRegisterImage_NilRequest(t *testing.T) {
	runner := &mockRunner{
		registerImageFunc: func(
			_ context.Context, _ string, _ *bool, _ *string, _ *string,
			_ *int, _ *int, _ *string, _ string,
		) error {
			return nil
		},
	}
	service := newImageTestService(t, runner)

	_, registerErr := service.RegisterImage(
		context.Background(),
		nil,
		"test@example.com",
	)

	assert.Error(t, registerErr)
	var appErr *apperrors.AppError
	assert.True(t, errors.As(registerErr, &appErr))
	assert.Contains(t, registerErr.Error(), "request is required")
	assert.Equal(t, apperrors.ErrCodeInvalidRequest, apperrors.GetErrorCode(registerErr))
	assert.Equal(t, http.StatusBadRequest, apperrors.GetStatusCode(registerErr))
}
