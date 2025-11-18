package orchestrator

import (
	"context"
	"errors"
	"testing"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterImage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		image         string
		isDefault     *bool
		runnerErr     error
		expectErr     bool
		expectedError string
	}{
		{
			name:      "successful registration",
			image:     "alpine:latest",
			isDefault: nil,
			runnerErr: nil,
			expectErr: false,
		},
		{
			name:      "successful registration with default flag true",
			image:     "ubuntu:22.04",
			isDefault: boolPtr(true),
			runnerErr: nil,
			expectErr: false,
		},
		{
			name:      "successful registration with default flag false",
			image:     "nginx:latest",
			isDefault: boolPtr(false),
			runnerErr: nil,
			expectErr: false,
		},
		{
			name:          "empty image name",
			image:         "",
			isDefault:     nil,
			expectErr:     true,
			expectedError: "image is required",
		},
		{
			name:          "runner error",
			image:         "alpine:latest",
			isDefault:     nil,
			runnerErr:     errors.New("failed to create task definition"),
			expectErr:     true,
			expectedError: "failed to register image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{
				registerImageFunc: func(
					_ context.Context, _ string, _ *bool, _ *string, _ *string, _ *int, _ *int, _ *string, _ string,
				) error {
					return tt.runnerErr
				},
			}

			svc := newTestService(nil, nil, runner)
			resp, err := svc.RegisterImage(
				ctx,
				&api.RegisterImageRequest{
					Image:     tt.image,
					IsDefault: tt.isDefault,
				},
				"test@example.com",
			)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.image, resp.Image)
				assert.Equal(t, "Image registered successfully", resp.Message)
			}
		})
	}
}

func TestListImages(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		mockImages  []api.ImageInfo
		runnerErr   error
		expectErr   bool
		expectedErr string
	}{
		{
			name: "successful list with images",
			mockImages: []api.ImageInfo{
				{Image: "alpine:latest", ImageID: "alpine:latest", RegisteredBy: "test@example.com", IsDefault: boolPtr(true)},
				{Image: "ubuntu:22.04", ImageID: "ubuntu:22.04", RegisteredBy: "test@example.com", IsDefault: boolPtr(false)},
			},
			runnerErr: nil,
			expectErr: false,
		},
		{
			name:       "successful list with empty images",
			mockImages: []api.ImageInfo{},
			runnerErr:  nil,
			expectErr:  false,
		},
		{
			name:        "runner error",
			mockImages:  nil,
			runnerErr:   errors.New("failed to describe task definitions"),
			expectErr:   true,
			expectedErr: "failed to list images",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			runner := &mockRunner{
				listImagesFunc: func(_ context.Context) ([]api.ImageInfo, error) {
					callCount++
					// For error cases, return empty list during initialization, error on test call
					if tt.expectErr && callCount == 1 {
						return []api.ImageInfo{}, nil
					}
					return tt.mockImages, tt.runnerErr
				},
			}

			svc := newTestService(nil, nil, runner)
			resp, err := svc.ListImages(ctx)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.mockImages, resp.Images)
			}
		})
	}
}

func TestRemoveImage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		image         string
		runnerErr     error
		expectErr     bool
		expectedError string
	}{
		{
			name:      "successful removal",
			image:     "alpine:latest",
			runnerErr: nil,
			expectErr: false,
		},
		{
			name:          "empty image name",
			image:         "",
			expectErr:     true,
			expectedError: "image is required",
		},
		{
			name:          "runner error",
			image:         "alpine:latest",
			runnerErr:     errors.New("failed to deregister task definition"),
			expectErr:     true,
			expectedError: "failed to remove image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{
				removeImageFunc: func(_ context.Context, _ string) error {
					return tt.runnerErr
				},
			}

			svc := newTestService(nil, nil, runner)
			err := svc.RemoveImage(ctx, tt.image)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)

				// Verify error type for validation errors
				if tt.image == "" {
					assert.Equal(t, apperrors.ErrCodeInvalidRequest, apperrors.GetErrorCode(err))
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
