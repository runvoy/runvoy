package dynamodb

import (
	"context"
	"fmt"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockImageClient is a mock implementation of the DynamoDB Client interface for testing.
type mockImageClient struct {
	putItemFunc    mockPutItemFunc
	getItemFunc    mockGetItemFunc
	queryFunc      mockQueryFunc
	updateItemFunc mockUpdateItemFunc
	deleteItemFunc mockDeleteItemFunc
	scanFunc       mockScanFunc
}

// Type aliases to reduce line length in function signatures
type mockPutItemFunc func(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (
	*dynamodb.PutItemOutput, error)
type mockGetItemFunc func(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (
	*dynamodb.GetItemOutput, error)
type mockQueryFunc func(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (
	*dynamodb.QueryOutput, error)
type mockUpdateItemFunc func(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (
	*dynamodb.UpdateItemOutput, error)
type mockDeleteItemFunc func(context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options)) (
	*dynamodb.DeleteItemOutput, error)
type mockScanFunc func(context.Context, *dynamodb.ScanInput, ...func(*dynamodb.Options)) (
	*dynamodb.ScanOutput, error)

func (m *mockImageClient) PutItem(
	ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (
	*dynamodb.PutItemOutput, error) {
	if m.putItemFunc != nil {
		return m.putItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (m *mockImageClient) GetItem(
	ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (
	*dynamodb.GetItemOutput, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *mockImageClient) Query(
	ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (
	*dynamodb.QueryOutput, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, params, optFns...)
	}
	return &dynamodb.QueryOutput{}, nil
}

func (m *mockImageClient) UpdateItem(
	ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (
	*dynamodb.UpdateItemOutput, error) {
	if m.updateItemFunc != nil {
		return m.updateItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (m *mockImageClient) DeleteItem(
	ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (
	*dynamodb.DeleteItemOutput, error) {
	if m.deleteItemFunc != nil {
		return m.deleteItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.DeleteItemOutput{}, nil
}

func (m *mockImageClient) BatchWriteItem(
	_ context.Context, _ *dynamodb.BatchWriteItemInput, _ ...func(*dynamodb.Options)) (
	*dynamodb.BatchWriteItemOutput, error) {
	return &dynamodb.BatchWriteItemOutput{}, nil
}

func (m *mockImageClient) Scan(
	ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (
	*dynamodb.ScanOutput, error) {
	if m.scanFunc != nil {
		return m.scanFunc(ctx, params, optFns...)
	}
	return &dynamodb.ScanOutput{}, nil
}

func TestBuildRoleComposite(t *testing.T) {
	tests := []struct {
		name                  string
		taskRoleName          *string
		taskExecutionRoleName *string
		expected              string
	}{
		{
			name:                  "both roles nil",
			taskRoleName:          nil,
			taskExecutionRoleName: nil,
			expected:              "default#default",
		},
		{
			name:                  "both roles empty strings",
			taskRoleName:          aws.String(""),
			taskExecutionRoleName: aws.String(""),
			expected:              "default#default",
		},
		{
			name:                  "only task role specified",
			taskRoleName:          aws.String("my-task-role"),
			taskExecutionRoleName: nil,
			expected:              "my-task-role#default",
		},
		{
			name:                  "only execution role specified",
			taskRoleName:          nil,
			taskExecutionRoleName: aws.String("my-exec-role"),
			expected:              "default#my-exec-role",
		},
		{
			name:                  "both roles specified",
			taskRoleName:          aws.String("my-task-role"),
			taskExecutionRoleName: aws.String("my-exec-role"),
			expected:              "my-task-role#my-exec-role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildRoleComposite(tt.taskRoleName, tt.taskExecutionRoleName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageTaskDefItem_IsDefault(t *testing.T) {
	tests := []struct {
		name     string
		item     *imageTaskDefItem
		expected bool
	}{
		{
			name: "default placeholder set",
			item: &imageTaskDefItem{
				IsDefaultPlaceholder: aws.String(defaultPlaceholderValue),
			},
			expected: true,
		},
		{
			name: "placeholder nil",
			item: &imageTaskDefItem{
				IsDefaultPlaceholder: nil,
			},
			expected: false,
		},
		{
			name: "placeholder set to different value",
			item: &imageTaskDefItem{
				IsDefaultPlaceholder: aws.String("NOT_DEFAULT"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.item.isDefault()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPutImageTaskDef(t *testing.T) {
	ctx := testutil.TestContext()
	logger := testutil.SilentLogger()

	tests := []struct {
		name                  string
		image                 string
		imageRegistry         string
		imageName             string
		imageTag              string
		taskRoleName          *string
		taskExecutionRoleName *string
		taskDefFamily         string
		isDefault             bool
		mockSetup             func(*mockImageClient)
		expectError           bool
	}{
		{
			name:                  "successful put with default image",
			image:                 "nginx:latest",
			imageRegistry:         "",
			imageName:             "nginx",
			imageTag:              "latest",
			taskRoleName:          nil,
			taskExecutionRoleName: nil,
			taskDefFamily:         "runvoy-taskdef-123",
			isDefault:             true,
			mockSetup: func(m *mockImageClient) {
				m.putItemFunc = func(
					_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.PutItemOutput, error) {
					// Verify the item was marshaled correctly
					assert.NotNil(t, params.Item)
					assert.Equal(t, "test-table", *params.TableName)

					// Unmarshal and verify fields
					var item imageTaskDefItem
					err := attributevalue.UnmarshalMap(params.Item, &item)
					require.NoError(t, err)

					assert.Equal(t, "nginx:latest", item.Image)
					assert.Equal(t, "default#default", item.RoleComposite)
					assert.Equal(t, "nginx", item.ImageName)
					assert.Equal(t, "latest", item.ImageTag)
					assert.Equal(t, "runvoy-taskdef-123", item.TaskDefinitionFamily)
					assert.NotNil(t, item.IsDefaultPlaceholder)
					assert.Equal(t, "DEFAULT", *item.IsDefaultPlaceholder)

					return &dynamodb.PutItemOutput{}, nil
				}
			},
			expectError: false,
		},
		{
			name:                  "successful put with custom roles",
			image:                 "myregistry.com/myapp:v1.0",
			imageRegistry:         "myregistry.com",
			imageName:             "myapp",
			imageTag:              "v1.0",
			taskRoleName:          aws.String("custom-task-role"),
			taskExecutionRoleName: aws.String("custom-exec-role"),
			taskDefFamily:         "runvoy-taskdef-456",
			isDefault:             false,
			mockSetup: func(m *mockImageClient) {
				m.putItemFunc = func(
					_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.PutItemOutput, error) {
					var item imageTaskDefItem
					err := attributevalue.UnmarshalMap(params.Item, &item)
					require.NoError(t, err)

					assert.Equal(t, "custom-task-role#custom-exec-role", item.RoleComposite)
					assert.Nil(t, item.IsDefaultPlaceholder)
					assert.Equal(t, "custom-task-role", *item.TaskRoleName)
					assert.Equal(t, "custom-exec-role", *item.TaskExecutionRoleName)

					return &dynamodb.PutItemOutput{}, nil
				}
			},
			expectError: false,
		},
		{
			name:          "dynamodb error",
			image:         "nginx:latest",
			imageRegistry: "",
			imageName:     "nginx",
			imageTag:      "latest",
			taskDefFamily: "runvoy-taskdef-789",
			isDefault:     false,
			mockSetup: func(m *mockImageClient) {
				m.putItemFunc = func(
					_ context.Context, _ *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.PutItemOutput, error) {
					return nil, fmt.Errorf("dynamodb error")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockImageClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			repo := NewImageTaskDefRepository(mockClient, "test-table", logger)
			err := repo.PutImageTaskDef(
				ctx,
				tt.image,
				tt.imageRegistry,
				tt.imageName,
				tt.imageTag,
				tt.taskRoleName,
				tt.taskExecutionRoleName,
				tt.taskDefFamily,
				tt.isDefault,
			)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetImageTaskDef(t *testing.T) {
	ctx := testutil.TestContext()
	logger := testutil.SilentLogger()

	tests := []struct {
		name                  string
		image                 string
		taskRoleName          *string
		taskExecutionRoleName *string
		mockSetup             func(*mockImageClient)
		expectNil             bool
		expectError           bool
		validateResult        func(*testing.T, *api.ImageInfo)
	}{
		{
			name:  "image found with default roles",
			image: "nginx:latest",
			mockSetup: func(m *mockImageClient) {
				m.getItemFunc = func(
					_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.GetItemOutput, error) {
					item := &imageTaskDefItem{
						Image:                "nginx:latest",
						RoleComposite:        "default#default",
						TaskDefinitionFamily: "runvoy-taskdef-123",
						IsDefaultPlaceholder: aws.String("DEFAULT"),
						ImageRegistry:        "",
						ImageName:            "nginx",
						ImageTag:             "latest",
					}
					av, _ := attributevalue.MarshalMap(item)
					return &dynamodb.GetItemOutput{Item: av}, nil
				}
			},
			validateResult: func(t *testing.T, info *api.ImageInfo) {
				assert.Equal(t, "nginx:latest", info.Image)
				assert.Equal(t, "runvoy-taskdef-123", info.TaskDefinitionName)
				assert.True(t, *info.IsDefault)
				assert.Equal(t, "nginx", info.ImageName)
				assert.Equal(t, "latest", info.ImageTag)
			},
		},
		{
			name:                  "image found with custom roles",
			image:                 "myapp:v1",
			taskRoleName:          aws.String("my-task-role"),
			taskExecutionRoleName: aws.String("my-exec-role"),
			mockSetup: func(m *mockImageClient) {
				m.getItemFunc = func(
					_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.GetItemOutput, error) {
					item := &imageTaskDefItem{
						Image:                 "myapp:v1",
						RoleComposite:         "my-task-role#my-exec-role",
						TaskRoleName:          aws.String("my-task-role"),
						TaskExecutionRoleName: aws.String("my-exec-role"),
						TaskDefinitionFamily:  "runvoy-taskdef-456",
						ImageRegistry:         "",
						ImageName:             "myapp",
						ImageTag:              "v1",
					}
					av, _ := attributevalue.MarshalMap(item)
					return &dynamodb.GetItemOutput{Item: av}, nil
				}
			},
			validateResult: func(t *testing.T, info *api.ImageInfo) {
				assert.Equal(t, "myapp:v1", info.Image)
				assert.Equal(t, "my-task-role", *info.TaskRoleName)
				assert.Equal(t, "my-exec-role", *info.TaskExecutionRoleName)
				assert.False(t, *info.IsDefault)
			},
		},
		{
			name:  "image not found",
			image: "nonexistent:latest",
			mockSetup: func(m *mockImageClient) {
				m.getItemFunc = func(
					_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.GetItemOutput, error) {
					return &dynamodb.GetItemOutput{Item: nil}, nil
				}
			},
			expectNil: true,
		},
		{
			name:  "dynamodb error",
			image: "nginx:latest",
			mockSetup: func(m *mockImageClient) {
				m.getItemFunc = func(
					_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.GetItemOutput, error) {
					return nil, fmt.Errorf("dynamodb error")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockImageClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			repo := NewImageTaskDefRepository(mockClient, "test-table", logger)
			result, err := repo.GetImageTaskDef(ctx, tt.image, tt.taskRoleName, tt.taskExecutionRoleName)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				if tt.validateResult != nil {
					tt.validateResult(t, result)
				}
			}
		})
	}
}

func TestListImages(t *testing.T) {
	ctx := testutil.TestContext()
	logger := testutil.SilentLogger()

	tests := []struct {
		name        string
		mockSetup   func(*mockImageClient)
		expectError bool
		validateFn  func(*testing.T, []api.ImageInfo)
	}{
		{
			name: "list multiple images",
			mockSetup: func(m *mockImageClient) {
				m.scanFunc = func(_ context.Context, _ *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.ScanOutput, error) {
					items := []imageTaskDefItem{
						{
							Image:                "nginx:latest",
							RoleComposite:        "default#default",
							TaskDefinitionFamily: "taskdef-1",
							ImageName:            "nginx",
							ImageTag:             "latest",
						},
						{
							Image:                "alpine:3.14",
							RoleComposite:        "default#default",
							TaskDefinitionFamily: "taskdef-2",
							ImageName:            "alpine",
							ImageTag:             "3.14",
						},
					}
					var av []map[string]types.AttributeValue
					for _, item := range items {
						itemMap, _ := attributevalue.MarshalMap(&item)
						av = append(av, itemMap)
					}
					return &dynamodb.ScanOutput{Items: av}, nil
				}
			},
			validateFn: func(t *testing.T, images []api.ImageInfo) {
				assert.Len(t, images, 2)
				// Results should be sorted by image name
				assert.Equal(t, "alpine:3.14", images[0].Image)
				assert.Equal(t, "nginx:latest", images[1].Image)
			},
		},
		{
			name: "empty list",
			mockSetup: func(m *mockImageClient) {
				m.scanFunc = func(_ context.Context, _ *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.ScanOutput, error) {
					return &dynamodb.ScanOutput{Items: []map[string]types.AttributeValue{}}, nil
				}
			},
			validateFn: func(t *testing.T, images []api.ImageInfo) {
				assert.Len(t, images, 0)
			},
		},
		{
			name: "dynamodb error",
			mockSetup: func(m *mockImageClient) {
				m.scanFunc = func(_ context.Context, _ *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.ScanOutput, error) {
					return nil, fmt.Errorf("scan error")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockImageClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			repo := NewImageTaskDefRepository(mockClient, "test-table", logger)
			images, err := repo.ListImages(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validateFn != nil {
					tt.validateFn(t, images)
				}
			}
		})
	}
}

func TestGetDefaultImage(t *testing.T) {
	ctx := testutil.TestContext()
	logger := testutil.SilentLogger()

	tests := []struct {
		name        string
		mockSetup   func(*mockImageClient)
		expectNil   bool
		expectError bool
		validateFn  func(*testing.T, *api.ImageInfo)
	}{
		{
			name: "default image found",
			mockSetup: func(m *mockImageClient) {
				m.queryFunc = func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.QueryOutput, error) {
					item := &imageTaskDefItem{
						Image:                "nginx:latest",
						RoleComposite:        "default#default",
						TaskDefinitionFamily: "taskdef-default",
						IsDefaultPlaceholder: aws.String("DEFAULT"),
						ImageName:            "nginx",
						ImageTag:             "latest",
					}
					av, _ := attributevalue.MarshalMap(item)
					return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{av}}, nil
				}
			},
			validateFn: func(t *testing.T, info *api.ImageInfo) {
				assert.Equal(t, "nginx:latest", info.Image)
				assert.True(t, *info.IsDefault)
			},
		},
		{
			name: "no default image",
			mockSetup: func(m *mockImageClient) {
				m.queryFunc = func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.QueryOutput, error) {
					return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil
				}
			},
			expectNil: true,
		},
		{
			name: "query error",
			mockSetup: func(m *mockImageClient) {
				m.queryFunc = func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.QueryOutput, error) {
					return nil, fmt.Errorf("query error")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockImageClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			repo := NewImageTaskDefRepository(mockClient, "test-table", logger)
			result, err := repo.GetDefaultImage(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectNil {
					assert.Nil(t, result)
				} else {
					require.NotNil(t, result)
					if tt.validateFn != nil {
						tt.validateFn(t, result)
					}
				}
			}
		})
	}
}

func TestDeleteImage(t *testing.T) {
	ctx := testutil.TestContext()
	logger := testutil.SilentLogger()

	tests := []struct {
		name        string
		image       string
		mockSetup   func(*mockImageClient)
		expectError bool
	}{
		{
			name:  "delete single role combination",
			image: "nginx:latest",
			mockSetup: func(m *mockImageClient) {
				m.queryFunc = func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.QueryOutput, error) {
					item := &imageTaskDefItem{
						Image:         "nginx:latest",
						RoleComposite: "default#default",
					}
					av, _ := attributevalue.MarshalMap(item)
					return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{av}}, nil
				}
				m.deleteItemFunc = func(_ context.Context, params *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.DeleteItemOutput, error) {
					assert.Equal(t, "test-table", *params.TableName)
					return &dynamodb.DeleteItemOutput{}, nil
				}
			},
			expectError: false,
		},
		{
			name:  "delete multiple role combinations",
			image: "myapp:v1",
			mockSetup: func(m *mockImageClient) {
				m.queryFunc = func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.QueryOutput, error) {
					items := []imageTaskDefItem{
						{Image: "myapp:v1", RoleComposite: "default#default"},
						{Image: "myapp:v1", RoleComposite: "role1#role2"},
						{Image: "myapp:v1", RoleComposite: "role3#role4"},
					}
					var av []map[string]types.AttributeValue
					for _, item := range items {
						itemMap, _ := attributevalue.MarshalMap(&item)
						av = append(av, itemMap)
					}
					return &dynamodb.QueryOutput{Items: av}, nil
				}
				deleteCount := 0
				m.deleteItemFunc = func(_ context.Context, _ *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.DeleteItemOutput, error) {
					deleteCount++
					return &dynamodb.DeleteItemOutput{}, nil
				}
			},
			expectError: false,
		},
		{
			name:  "query error",
			image: "nginx:latest",
			mockSetup: func(m *mockImageClient) {
				m.queryFunc = func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.QueryOutput, error) {
					return nil, fmt.Errorf("query failed")
				}
			},
			expectError: true,
		},
		{
			name:  "delete error",
			image: "nginx:latest",
			mockSetup: func(m *mockImageClient) {
				m.queryFunc = func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.QueryOutput, error) {
					item := &imageTaskDefItem{
						Image:         "nginx:latest",
						RoleComposite: "default#default",
					}
					av, _ := attributevalue.MarshalMap(item)
					return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{av}}, nil
				}
				m.deleteItemFunc = func(_ context.Context, _ *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.DeleteItemOutput, error) {
					return nil, fmt.Errorf("delete failed")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockImageClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			repo := NewImageTaskDefRepository(mockClient, "test-table", logger)
			err := repo.DeleteImage(ctx, tt.image)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetUniqueImages(t *testing.T) {
	ctx := testutil.TestContext()
	logger := testutil.SilentLogger()

	tests := []struct {
		name        string
		mockSetup   func(*mockImageClient)
		expectError bool
		validateFn  func(*testing.T, []string)
	}{
		{
			name: "deduplicate images across role combinations",
			mockSetup: func(m *mockImageClient) {
				m.scanFunc = func(_ context.Context, _ *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.ScanOutput, error) {
					items := []imageTaskDefItem{
						{Image: "nginx:latest", RoleComposite: "default#default", ImageName: "nginx", ImageTag: "latest"},
						{Image: "nginx:latest", RoleComposite: "role1#role2", ImageName: "nginx", ImageTag: "latest"},
						{Image: "alpine:3.14", RoleComposite: "default#default", ImageName: "alpine", ImageTag: "3.14"},
						{Image: "alpine:3.14", RoleComposite: "role3#role4", ImageName: "alpine", ImageTag: "3.14"},
					}
					var av []map[string]types.AttributeValue
					for _, item := range items {
						itemMap, _ := attributevalue.MarshalMap(&item)
						av = append(av, itemMap)
					}
					return &dynamodb.ScanOutput{Items: av}, nil
				}
			},
			validateFn: func(t *testing.T, images []string) {
				// Should only have 2 unique images, sorted alphabetically
				assert.Len(t, images, 2)
				assert.Equal(t, "alpine:3.14", images[0])
				assert.Equal(t, "nginx:latest", images[1])
			},
		},
		{
			name: "empty list",
			mockSetup: func(m *mockImageClient) {
				m.scanFunc = func(_ context.Context, _ *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.ScanOutput, error) {
					return &dynamodb.ScanOutput{Items: []map[string]types.AttributeValue{}}, nil
				}
			},
			validateFn: func(t *testing.T, images []string) {
				assert.Len(t, images, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockImageClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			repo := NewImageTaskDefRepository(mockClient, "test-table", logger)
			images, err := repo.GetUniqueImages(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validateFn != nil {
					tt.validateFn(t, images)
				}
			}
		})
	}
}

func TestGetImagesCount(t *testing.T) {
	ctx := testutil.TestContext()
	logger := testutil.SilentLogger()

	tests := []struct {
		name          string
		mockSetup     func(*mockImageClient)
		expectedCount int
		expectError   bool
	}{
		{
			name: "count multiple images",
			mockSetup: func(m *mockImageClient) {
				m.scanFunc = func(_ context.Context, params *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.ScanOutput, error) {
					assert.Equal(t, types.SelectCount, params.Select)
					return &dynamodb.ScanOutput{Count: 5}, nil
				}
			},
			expectedCount: 5,
		},
		{
			name: "empty table",
			mockSetup: func(m *mockImageClient) {
				m.scanFunc = func(_ context.Context, _ *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.ScanOutput, error) {
					return &dynamodb.ScanOutput{Count: 0}, nil
				}
			},
			expectedCount: 0,
		},
		{
			name: "scan error",
			mockSetup: func(m *mockImageClient) {
				m.scanFunc = func(_ context.Context, _ *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.ScanOutput, error) {
					return nil, fmt.Errorf("scan failed")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockImageClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			repo := NewImageTaskDefRepository(mockClient, "test-table", logger)
			count, err := repo.GetImagesCount(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, count)
			}
		})
	}
}

func TestSetImageAsOnlyDefault(t *testing.T) {
	ctx := testutil.TestContext()
	logger := testutil.SilentLogger()

	tests := []struct {
		name        string
		image       string
		mockSetup   func(*mockImageClient)
		expectError bool
	}{
		{
			name:  "successfully set as only default",
			image: "nginx:latest",
			mockSetup: func(m *mockImageClient) {
				// Mock UnmarkAllDefaults (Query + UpdateItem for each existing default)
				m.queryFunc = func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.QueryOutput, error) {
					// Return one existing default to unmark
					item := &imageTaskDefItem{
						Image:                "alpine:3.14",
						RoleComposite:        "default#default",
						IsDefaultPlaceholder: aws.String("DEFAULT"),
					}
					av, _ := attributevalue.MarshalMap(item)
					return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{av}}, nil
				}
				m.updateItemFunc = func(_ context.Context, _ *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.UpdateItemOutput, error) {
					return &dynamodb.UpdateItemOutput{}, nil
				}
			},
			expectError: false,
		},
		{
			name:  "error unmarking defaults",
			image: "nginx:latest",
			mockSetup: func(m *mockImageClient) {
				m.queryFunc = func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (
					*dynamodb.QueryOutput, error) {
					return nil, fmt.Errorf("query failed")
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockImageClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			repo := NewImageTaskDefRepository(mockClient, "test-table", logger)
			err := repo.SetImageAsOnlyDefault(ctx, tt.image, nil, nil)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
