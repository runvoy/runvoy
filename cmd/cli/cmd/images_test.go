package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"runvoy/internal/api"
)

// mockClientInterfaceForImages extends mockClientInterface with image management methods
type mockClientInterfaceForImages struct {
	*mockClientInterface
	registerImageFunc func(
		ctx context.Context,
		image string,
		isDefault *bool,
		taskRoleName, taskExecutionRoleName *string,
		cpu, memory *int,
		runtimePlatform *string,
	) (*api.RegisterImageResponse, error)
	listImagesFunc      func(ctx context.Context) (*api.ListImagesResponse, error)
	unregisterImageFunc func(ctx context.Context, image string) (*api.RemoveImageResponse, error)
}

func (m *mockClientInterfaceForImages) RegisterImage(
	ctx context.Context, image string, isDefault *bool, taskRoleName, taskExecutionRoleName *string,
	cpu, memory *int,
	runtimePlatform *string,
) (*api.RegisterImageResponse, error) {
	if m.registerImageFunc != nil {
		return m.registerImageFunc(ctx, image, isDefault, taskRoleName, taskExecutionRoleName, cpu, memory, runtimePlatform)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForImages) ListImages(ctx context.Context) (*api.ListImagesResponse, error) {
	if m.listImagesFunc != nil {
		return m.listImagesFunc(ctx)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForImages) UnregisterImage(
	ctx context.Context, image string,
) (*api.RemoveImageResponse, error) {
	if m.unregisterImageFunc != nil {
		return m.unregisterImageFunc(ctx, image)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForImages) FetchBackendLogs(_ context.Context, _ string) (*api.TraceResponse, error) {
	return nil, nil
}

func TestImagesService_RegisterImage(t *testing.T) {
	tests := []struct {
		name                  string
		image                 string
		isDefault             *bool
		taskRoleName          *string
		taskExecutionRoleName *string
		setupMock             func(*mockClientInterfaceForImages)
		wantErr               bool
		verifyOutput          func(*testing.T, *mockOutputInterface)
	}{
		{
			name:      "successfully registers image",
			image:     "alpine:latest",
			isDefault: nil,
			setupMock: func(m *mockClientInterfaceForImages) {
				m.registerImageFunc = func(
					_ context.Context, image string, isDefault *bool, taskRoleName,
					taskExecutionRoleName *string, _, _ *int, _ *string,
				) (*api.RegisterImageResponse, error) {
					assert.Equal(t, "alpine:latest", image)
					assert.Nil(t, isDefault)
					assert.Nil(t, taskRoleName)
					assert.Nil(t, taskExecutionRoleName)
					return &api.RegisterImageResponse{
						Image:   "alpine:latest",
						Message: "Image registered",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasSuccess := false
				hasKeyValue := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
					}
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Image" {
							hasKeyValue = true
						}
					}
				}
				assert.True(t, hasSuccess, "Expected Successf call")
				assert.True(t, hasKeyValue, "Expected KeyValue call")
			},
		},
		{
			name:      "registers image as default",
			image:     "ubuntu:22.04",
			isDefault: func() *bool { b := true; return &b }(),
			setupMock: func(m *mockClientInterfaceForImages) {
				m.registerImageFunc = func(
					_ context.Context, image string, isDefault *bool, taskRoleName,
					taskExecutionRoleName *string, _, _ *int, _ *string,
				) (*api.RegisterImageResponse, error) {
					assert.Equal(t, "ubuntu:22.04", image)
					assert.NotNil(t, isDefault)
					assert.True(t, *isDefault)
					assert.Nil(t, taskRoleName)
					assert.Nil(t, taskExecutionRoleName)
					return &api.RegisterImageResponse{
						Image:   "ubuntu:22.04",
						Message: "Image registered as default",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasMessage := false
				for _, call := range m.calls {
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Message" {
							hasMessage = true
						}
					}
				}
				assert.True(t, hasMessage, "Expected Message KeyValue call")
			},
		},
		{
			name:                  "registers image with task roles",
			image:                 "alpine:latest",
			isDefault:             nil,
			taskRoleName:          func() *string { s := "my-task-role"; return &s }(),
			taskExecutionRoleName: func() *string { s := "my-exec-role"; return &s }(),
			setupMock: func(m *mockClientInterfaceForImages) {
				m.registerImageFunc = func(
					_ context.Context, image string, isDefault *bool, taskRoleName,
					taskExecutionRoleName *string, _, _ *int, _ *string,
				) (*api.RegisterImageResponse, error) {
					assert.Equal(t, "alpine:latest", image)
					assert.Nil(t, isDefault)
					assert.NotNil(t, taskRoleName)
					assert.Equal(t, "my-task-role", *taskRoleName)
					assert.NotNil(t, taskExecutionRoleName)
					assert.Equal(t, "my-exec-role", *taskExecutionRoleName)
					return &api.RegisterImageResponse{
						Image:   "alpine:latest",
						Message: "Image registered with custom roles",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.True(t, hasSuccess, "Expected Successf call")
			},
		},
		{
			name:      "handles client error",
			image:     "invalid:image",
			isDefault: nil,
			setupMock: func(m *mockClientInterfaceForImages) {
				m.registerImageFunc = func(
					_ context.Context, _ string, _ *bool, _, _ *string, _, _ *int, _ *string,
				) (*api.RegisterImageResponse, error) {
					return nil, fmt.Errorf("invalid image format")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.False(t, hasSuccess, "Should not have Successf on error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForImages{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewImagesService(mockClient, mockOutput)

			err := service.RegisterImage(
				context.Background(), tt.image, tt.isDefault, tt.taskRoleName, tt.taskExecutionRoleName, nil, nil, nil,
			)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.verifyOutput != nil {
				tt.verifyOutput(t, mockOutput)
			}
		})
	}
}

func TestImagesService_ListImages(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*mockClientInterfaceForImages)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name: "successfully lists images",
			setupMock: func(m *mockClientInterfaceForImages) {
				m.listImagesFunc = func(_ context.Context) (*api.ListImagesResponse, error) {
					isDefaultTrue := true
					isDefaultFalse := false
					return &api.ListImagesResponse{
						Images: []api.ImageInfo{
							{
								Image:     "alpine:latest",
								IsDefault: &isDefaultTrue,
							},
							{
								Image:     "ubuntu:22.04",
								IsDefault: &isDefaultFalse,
							},
						},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasTable := false
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Table" {
						hasTable = true
						if len(call.args) >= 2 {
							rows := call.args[1].([][]string)
							assert.Equal(t, 2, len(rows), "Should have 2 image rows")
						}
					}
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.True(t, hasTable, "Expected Table call")
				assert.True(t, hasSuccess, "Expected Successf call")
			},
		},
		{
			name: "handles empty image list",
			setupMock: func(m *mockClientInterfaceForImages) {
				m.listImagesFunc = func(_ context.Context) (*api.ListImagesResponse, error) {
					return &api.ListImagesResponse{
						Images: []api.ImageInfo{},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasTable := false
				for _, call := range m.calls {
					if call.method == "Table" {
						hasTable = true
						if len(call.args) >= 2 {
							rows := call.args[1].([][]string)
							assert.Equal(t, 0, len(rows), "Should have empty rows")
						}
					}
				}
				assert.True(t, hasTable, "Expected Table call even with empty list")
			},
		},
		{
			name: "handles client error",
			setupMock: func(m *mockClientInterfaceForImages) {
				m.listImagesFunc = func(_ context.Context) (*api.ListImagesResponse, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasTable := false
				for _, call := range m.calls {
					if call.method == "Table" {
						hasTable = true
					}
				}
				assert.False(t, hasTable, "Should not call Table on error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForImages{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewImagesService(mockClient, mockOutput)

			err := service.ListImages(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.verifyOutput != nil {
				tt.verifyOutput(t, mockOutput)
			}
		})
	}
}

func TestImagesService_UnregisterImage(t *testing.T) {
	tests := []struct {
		name         string
		image        string
		setupMock    func(*mockClientInterfaceForImages)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:  "successfully unregisters image",
			image: "alpine:latest",
			setupMock: func(m *mockClientInterfaceForImages) {
				m.unregisterImageFunc = func(_ context.Context, image string) (*api.RemoveImageResponse, error) {
					assert.Equal(t, "alpine:latest", image)
					return &api.RemoveImageResponse{
						Image:   "alpine:latest",
						Message: "Image removed successfully",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasSuccess := false
				hasKeyValue := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
					}
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Image" || call.args[0] == "Message" {
							hasKeyValue = true
						}
					}
				}
				assert.True(t, hasSuccess, "Expected Successf call")
				assert.True(t, hasKeyValue, "Expected KeyValue calls")
			},
		},
		{
			name:  "handles image not found error",
			image: "nonexistent:latest",
			setupMock: func(m *mockClientInterfaceForImages) {
				m.unregisterImageFunc = func(_ context.Context, _ string) (*api.RemoveImageResponse, error) {
					return nil, fmt.Errorf("image not found")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.False(t, hasSuccess, "Should not have Successf on error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForImages{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewImagesService(mockClient, mockOutput)

			err := service.UnregisterImage(context.Background(), tt.image)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.verifyOutput != nil {
				tt.verifyOutput(t, mockOutput)
			}
		})
	}
}
