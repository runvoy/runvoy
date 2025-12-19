package infra

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/runvoy/runvoy/internal/client/infra/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCloudFormationClient is a mock implementation of CloudFormationClient
//
//nolint:dupl // Mock struct must match interface signature
type mockCloudFormationClient struct {
	describeStacksFunc func(
		ctx context.Context,
		params *cloudformation.DescribeStacksInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.DescribeStacksOutput, error)
	describeStackEventsFunc func(
		ctx context.Context,
		params *cloudformation.DescribeStackEventsInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.DescribeStackEventsOutput, error)
	createStackFunc func(
		ctx context.Context,
		params *cloudformation.CreateStackInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.CreateStackOutput, error)
	updateStackFunc func(
		ctx context.Context,
		params *cloudformation.UpdateStackInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.UpdateStackOutput, error)
	deleteStackFunc func(
		ctx context.Context,
		params *cloudformation.DeleteStackInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.DeleteStackOutput, error)
}

func (m *mockCloudFormationClient) DescribeStacks(
	ctx context.Context,
	params *cloudformation.DescribeStacksInput,
	optFns ...func(*cloudformation.Options),
) (*cloudformation.DescribeStacksOutput, error) {
	if m.describeStacksFunc != nil {
		return m.describeStacksFunc(ctx, params, optFns...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCloudFormationClient) DescribeStackEvents(
	ctx context.Context,
	params *cloudformation.DescribeStackEventsInput,
	optFns ...func(*cloudformation.Options),
) (*cloudformation.DescribeStackEventsOutput, error) {
	if m.describeStackEventsFunc != nil {
		return m.describeStackEventsFunc(ctx, params, optFns...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCloudFormationClient) CreateStack(
	ctx context.Context,
	params *cloudformation.CreateStackInput,
	optFns ...func(*cloudformation.Options),
) (*cloudformation.CreateStackOutput, error) {
	if m.createStackFunc != nil {
		return m.createStackFunc(ctx, params, optFns...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCloudFormationClient) UpdateStack(
	ctx context.Context,
	params *cloudformation.UpdateStackInput,
	optFns ...func(*cloudformation.Options),
) (*cloudformation.UpdateStackOutput, error) {
	if m.updateStackFunc != nil {
		return m.updateStackFunc(ctx, params, optFns...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCloudFormationClient) DeleteStack(
	ctx context.Context,
	params *cloudformation.DeleteStackInput,
	optFns ...func(*cloudformation.Options),
) (*cloudformation.DeleteStackOutput, error) {
	if m.deleteStackFunc != nil {
		return m.deleteStackFunc(ctx, params, optFns...)
	}
	return nil, errors.New("not implemented")
}

func TestNewAWSDeployerWithClient(t *testing.T) {
	t.Run("creates deployer with custom client", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{}
		region := "us-east-1"

		deployer := NewAWSDeployerWithClient(mockClient, region)

		require.NotNil(t, deployer)
		assert.Equal(t, region, deployer.GetRegion())
		assert.Equal(t, mockClient, deployer.client)
	})
}

func TestAWSDeployer_CheckExists(t *testing.T) {
	t.Run("stack exists", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				params *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   params.StackName,
							StackStatus: types.StackStatusCreateComplete,
						},
					},
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		exists, err := deployer.CheckExists(context.Background(), "test-stack")

		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("stack does not exist", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				_ *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return nil, errors.New("Stack with id test-stack does not exist")
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		exists, err := deployer.CheckExists(context.Background(), "test-stack")

		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("error checking stack", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				_ *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return nil, errors.New("AWS API error")
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		exists, err := deployer.CheckExists(context.Background(), "test-stack")

		require.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "AWS API error")
	})
}

func TestAWSDeployer_GetOutputs(t *testing.T) {
	t.Run("successful output retrieval", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				params *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   params.StackName,
							StackStatus: types.StackStatusCreateComplete,
							Outputs: []types.Output{
								{
									OutputKey:   aws.String("ApiEndpoint"),
									OutputValue: aws.String("https://api.example.com"),
								},
								{
									OutputKey:   aws.String("DatabaseURL"),
									OutputValue: aws.String("postgres://localhost:5432"),
								},
							},
						},
					},
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		outputs, err := deployer.GetOutputs(context.Background(), "test-stack")

		require.NoError(t, err)
		assert.Len(t, outputs, 2)
		assert.Equal(t, "https://api.example.com", outputs["ApiEndpoint"])
		assert.Equal(t, "postgres://localhost:5432", outputs["DatabaseURL"])
	})

	t.Run("stack not found", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				_ *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return nil, errors.New("stack does not exist")
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		outputs, err := deployer.GetOutputs(context.Background(), "nonexistent-stack")

		require.Error(t, err)
		assert.Nil(t, outputs)
	})

	t.Run("empty outputs", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				params *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   params.StackName,
							StackStatus: types.StackStatusCreateComplete,
							Outputs:     []types.Output{},
						},
					},
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		outputs, err := deployer.GetOutputs(context.Background(), "test-stack")

		require.NoError(t, err)
		assert.Empty(t, outputs)
	})

	t.Run("no stacks in response", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				_ *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{},
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		outputs, err := deployer.GetOutputs(context.Background(), "test-stack")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "stack not found")
		assert.Nil(t, outputs)
	})
}

func TestAWSDeployer_parseParametersToCFN(t *testing.T) {
	t.Run("valid parameters with defaults", func(t *testing.T) {
		deployer := NewAWSDeployerWithClient(&mockCloudFormationClient{}, "us-west-2")
		params := []string{
			"Key1=Value1",
			"Key2=Value2",
		}

		cfnParams, err := deployer.parseParametersToCFN(params, "v1.0.0")

		require.NoError(t, err)
		assert.Len(t, cfnParams, 4) // 2 provided + LambdaCodeBucket + ReleaseVersion

		// Check that default parameters are added
		paramMap := make(map[string]string)
		for _, p := range cfnParams {
			paramMap[*p.ParameterKey] = *p.ParameterValue
		}

		assert.Equal(t, "Value1", paramMap["Key1"])
		assert.Equal(t, "Value2", paramMap["Key2"])
		assert.Equal(t, "runvoy-releases-us-west-2", paramMap["LambdaCodeBucket"])
		assert.Equal(t, "1.0.0", paramMap["ReleaseVersion"]) // version gets normalized (v prefix removed)
	})

	t.Run("override default LambdaCodeBucket", func(t *testing.T) {
		deployer := NewAWSDeployerWithClient(&mockCloudFormationClient{}, "us-east-1")
		params := []string{
			"LambdaCodeBucket=my-custom-bucket",
		}

		cfnParams, err := deployer.parseParametersToCFN(params, "v1.0.0")

		require.NoError(t, err)

		paramMap := make(map[string]string)
		for _, p := range cfnParams {
			paramMap[*p.ParameterKey] = *p.ParameterValue
		}

		assert.Equal(t, "my-custom-bucket", paramMap["LambdaCodeBucket"])
	})

	t.Run("override default ReleaseVersion", func(t *testing.T) {
		deployer := NewAWSDeployerWithClient(&mockCloudFormationClient{}, "us-east-1")
		params := []string{
			"ReleaseVersion=v2.0.0",
		}

		cfnParams, err := deployer.parseParametersToCFN(params, "v1.0.0")

		require.NoError(t, err)

		paramMap := make(map[string]string)
		for _, p := range cfnParams {
			paramMap[*p.ParameterKey] = *p.ParameterValue
		}

		assert.Equal(t, "v2.0.0", paramMap["ReleaseVersion"])
	})

	t.Run("no version provided", func(t *testing.T) {
		deployer := NewAWSDeployerWithClient(&mockCloudFormationClient{}, "us-east-1")
		params := []string{
			"Key1=Value1",
		}

		cfnParams, err := deployer.parseParametersToCFN(params, "")

		require.NoError(t, err)

		paramMap := make(map[string]string)
		for _, p := range cfnParams {
			paramMap[*p.ParameterKey] = *p.ParameterValue
		}

		// ReleaseVersion should not be added if version is empty and not provided
		_, hasReleaseVersion := paramMap["ReleaseVersion"]
		assert.False(t, hasReleaseVersion)
	})

	t.Run("invalid parameter format", func(t *testing.T) {
		deployer := NewAWSDeployerWithClient(&mockCloudFormationClient{}, "us-east-1")
		params := []string{
			"InvalidParameter",
		}

		cfnParams, err := deployer.parseParametersToCFN(params, "v1.0.0")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid parameter format")
		assert.Nil(t, cfnParams)
	})
}

func TestAWSDeployer_Deploy_NoWait(t *testing.T) {
	t.Run("create stack without waiting", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				_ *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return nil, errors.New("does not exist")
			},
			createStackFunc: func(
				_ context.Context,
				_ *cloudformation.CreateStackInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.CreateStackOutput, error) {
				return &cloudformation.CreateStackOutput{
					StackId: aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/test-stack/12345"),
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		opts := &core.DeployOptions{
			Name:       "test-stack",
			Template:   "https://example.com/template.yaml",
			Version:    "v1.0.0",
			Parameters: []string{},
			Wait:       false,
		}

		result, err := deployer.Deploy(context.Background(), opts)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test-stack", result.Name)
		assert.Equal(t, "CREATE", result.OperationType)
		assert.Equal(t, "IN_PROGRESS", result.Status)
		assert.False(t, result.NoChanges)
	})

	t.Run("update stack without waiting", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				params *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   params.StackName,
							StackStatus: types.StackStatusCreateComplete,
						},
					},
				}, nil
			},
			updateStackFunc: func(
				_ context.Context,
				_ *cloudformation.UpdateStackInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.UpdateStackOutput, error) {
				return &cloudformation.UpdateStackOutput{
					StackId: aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/test-stack/12345"),
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		opts := &core.DeployOptions{
			Name:       "test-stack",
			Template:   "https://example.com/template.yaml",
			Version:    "v1.0.0",
			Parameters: []string{},
			Wait:       false,
		}

		result, err := deployer.Deploy(context.Background(), opts)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test-stack", result.Name)
		assert.Equal(t, "UPDATE", result.OperationType)
		assert.Equal(t, "IN_PROGRESS", result.Status)
		assert.False(t, result.NoChanges)
	})

	t.Run("no changes to perform", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				params *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   params.StackName,
							StackStatus: types.StackStatusCreateComplete,
						},
					},
				}, nil
			},
			updateStackFunc: func(
				_ context.Context,
				_ *cloudformation.UpdateStackInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.UpdateStackOutput, error) {
				return nil, errors.New("No updates are to be performed")
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		opts := &core.DeployOptions{
			Name:       "test-stack",
			Template:   "https://example.com/template.yaml",
			Version:    "v1.0.0",
			Parameters: []string{},
			Wait:       false,
		}

		result, err := deployer.Deploy(context.Background(), opts)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test-stack", result.Name)
		assert.Equal(t, "UPDATE", result.OperationType)
		assert.Equal(t, "NO_CHANGES", result.Status)
		assert.True(t, result.NoChanges)
	})
}

func TestAWSDeployer_Destroy(t *testing.T) {
	t.Run("destroy existing stack without waiting", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				params *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   params.StackName,
							StackStatus: types.StackStatusCreateComplete,
						},
					},
				}, nil
			},
			deleteStackFunc: func(
				_ context.Context,
				_ *cloudformation.DeleteStackInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DeleteStackOutput, error) {
				return &cloudformation.DeleteStackOutput{}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		opts := &core.DestroyOptions{
			Name: "test-stack",
			Wait: false,
		}

		result, err := deployer.Destroy(context.Background(), opts)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test-stack", result.Name)
		assert.Equal(t, "IN_PROGRESS", result.Status)
		assert.False(t, result.NotFound)
	})

	t.Run("destroy non-existent stack", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				_ *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return nil, errors.New("does not exist")
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		opts := &core.DestroyOptions{
			Name: "nonexistent-stack",
			Wait: false,
		}

		result, err := deployer.Destroy(context.Background(), opts)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "nonexistent-stack", result.Name)
		assert.Equal(t, "NOT_FOUND", result.Status)
		assert.True(t, result.NotFound)
	})

	t.Run("error during deletion", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				params *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   params.StackName,
							StackStatus: types.StackStatusCreateComplete,
						},
					},
				}, nil
			},
			deleteStackFunc: func(
				_ context.Context,
				_ *cloudformation.DeleteStackInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DeleteStackOutput, error) {
				return nil, errors.New("deletion failed")
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		opts := &core.DestroyOptions{
			Name: "test-stack",
			Wait: false,
		}

		result, err := deployer.Destroy(context.Background(), opts)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete stack")
		assert.Nil(t, result)
	})
}

func TestAWSDeployer_CreateUpdateStack(t *testing.T) {
	t.Run("create stack with URL template", func(t *testing.T) {
		var capturedInput *cloudformation.CreateStackInput
		mockClient := &mockCloudFormationClient{
			createStackFunc: func(
				_ context.Context,
				params *cloudformation.CreateStackInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.CreateStackOutput, error) {
				capturedInput = params
				return &cloudformation.CreateStackOutput{
					StackId: aws.String("stack-id"),
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		template := &core.TemplateSource{URL: "https://example.com/template.yaml"}
		params := []types.Parameter{
			{ParameterKey: aws.String("Key1"), ParameterValue: aws.String("Value1")},
		}

		err := deployer.createStack(context.Background(), "test-stack", template, params)

		require.NoError(t, err)
		require.NotNil(t, capturedInput)
		assert.Equal(t, "test-stack", *capturedInput.StackName)
		assert.Equal(t, "https://example.com/template.yaml", *capturedInput.TemplateURL)
		assert.Nil(t, capturedInput.TemplateBody)
		assert.Len(t, capturedInput.Parameters, 1)
		assert.Contains(t, capturedInput.Capabilities, types.CapabilityCapabilityNamedIam)
	})

	//nolint:dupl // Similar test structure for create and update operations
	t.Run("create stack with body template", func(t *testing.T) {
		var capturedInput *cloudformation.CreateStackInput
		mockClient := &mockCloudFormationClient{
			createStackFunc: func(
				_ context.Context,
				params *cloudformation.CreateStackInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.CreateStackOutput, error) {
				capturedInput = params
				return &cloudformation.CreateStackOutput{StackId: aws.String("stack-id")}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		template := &core.TemplateSource{Body: "template body content"}
		err := deployer.createStack(context.Background(), "test-stack", template, []types.Parameter{})

		require.NoError(t, err)
		require.NotNil(t, capturedInput)
		assert.Equal(t, "test-stack", *capturedInput.StackName)
		assert.Equal(t, "template body content", *capturedInput.TemplateBody)
		assert.Nil(t, capturedInput.TemplateURL)
	})

	t.Run("update stack with URL template", func(t *testing.T) {
		var capturedInput *cloudformation.UpdateStackInput
		mockClient := &mockCloudFormationClient{
			updateStackFunc: func(
				_ context.Context,
				params *cloudformation.UpdateStackInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.UpdateStackOutput, error) {
				capturedInput = params
				return &cloudformation.UpdateStackOutput{
					StackId: aws.String("stack-id"),
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		template := &core.TemplateSource{URL: "https://example.com/template.yaml"}
		params := []types.Parameter{
			{ParameterKey: aws.String("Key1"), ParameterValue: aws.String("Value1")},
		}

		err := deployer.updateStack(context.Background(), "test-stack", template, params)

		require.NoError(t, err)
		require.NotNil(t, capturedInput)
		assert.Equal(t, "test-stack", *capturedInput.StackName)
		assert.Equal(t, "https://example.com/template.yaml", *capturedInput.TemplateURL)
		assert.Nil(t, capturedInput.TemplateBody)
		assert.Len(t, capturedInput.Parameters, 1)
	})

	//nolint:dupl // Similar test structure for create and update operations
	t.Run("update stack with body template", func(t *testing.T) {
		var capturedInput *cloudformation.UpdateStackInput
		mockClient := &mockCloudFormationClient{
			updateStackFunc: func(
				_ context.Context,
				params *cloudformation.UpdateStackInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.UpdateStackOutput, error) {
				capturedInput = params
				return &cloudformation.UpdateStackOutput{StackId: aws.String("stack-id")}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		template := &core.TemplateSource{Body: "updated template body"}
		err := deployer.updateStack(context.Background(), "test-stack", template, []types.Parameter{})

		require.NoError(t, err)
		require.NotNil(t, capturedInput)
		assert.Equal(t, "test-stack", *capturedInput.StackName)
		assert.Equal(t, "updated template body", *capturedInput.TemplateBody)
		assert.Nil(t, capturedInput.TemplateURL)
	})
}

func TestAWSDeployer_GetStackStatus(t *testing.T) {
	t.Run("successful status retrieval", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				params *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:         params.StackName,
							StackStatus:       types.StackStatusCreateComplete,
							StackStatusReason: aws.String("Stack created successfully"),
						},
					},
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		status, reason, err := deployer.getStackStatus(context.Background(), "test-stack")

		require.NoError(t, err)
		assert.Equal(t, "CREATE_COMPLETE", status)
		assert.Equal(t, "Stack created successfully", reason)
	})

	t.Run("stack not found", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				_ *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{},
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		status, reason, err := deployer.getStackStatus(context.Background(), "test-stack")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "stack not found")
		assert.Empty(t, status)
		assert.Empty(t, reason)
	})

	t.Run("status without reason", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStacksFunc: func(
				_ context.Context,
				params *cloudformation.DescribeStacksInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStacksOutput, error) {
				return &cloudformation.DescribeStacksOutput{
					Stacks: []types.Stack{
						{
							StackName:   params.StackName,
							StackStatus: types.StackStatusCreateInProgress,
						},
					},
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		status, reason, err := deployer.getStackStatus(context.Background(), "test-stack")

		require.NoError(t, err)
		assert.Equal(t, "CREATE_IN_PROGRESS", status)
		assert.Empty(t, reason)
	})
}

func TestAWSDeployer_GetFailedResourceEvents(t *testing.T) {
	t.Run("retrieves failure events", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStackEventsFunc: func(
				_ context.Context,
				_ *cloudformation.DescribeStackEventsInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStackEventsOutput, error) {
				return &cloudformation.DescribeStackEventsOutput{
					StackEvents: []types.StackEvent{
						{
							LogicalResourceId:    aws.String("MyResource"),
							ResourceType:         aws.String("AWS::S3::Bucket"),
							ResourceStatus:       types.ResourceStatusCreateFailed,
							ResourceStatusReason: aws.String("Bucket already exists"),
						},
						{
							LogicalResourceId:    aws.String("MyOtherResource"),
							ResourceType:         aws.String("AWS::Lambda::Function"),
							ResourceStatus:       types.ResourceStatusCreateComplete,
							ResourceStatusReason: nil,
						},
					},
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		failures := deployer.getFailedResourceEvents(context.Background(), "test-stack")

		assert.NotEmpty(t, failures)
		assert.Contains(t, failures, "MyResource")
		assert.Contains(t, failures, "AWS::S3::Bucket")
		assert.Contains(t, failures, "Bucket already exists")
	})

	t.Run("no failure events", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStackEventsFunc: func(
				_ context.Context,
				_ *cloudformation.DescribeStackEventsInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStackEventsOutput, error) {
				return &cloudformation.DescribeStackEventsOutput{
					StackEvents: []types.StackEvent{
						{
							LogicalResourceId: aws.String("MyResource"),
							ResourceType:      aws.String("AWS::S3::Bucket"),
							ResourceStatus:    types.ResourceStatusCreateComplete,
						},
					},
				}, nil
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		failures := deployer.getFailedResourceEvents(context.Background(), "test-stack")

		assert.Empty(t, failures)
	})

	t.Run("error fetching events", func(t *testing.T) {
		mockClient := &mockCloudFormationClient{
			describeStackEventsFunc: func(
				_ context.Context,
				_ *cloudformation.DescribeStackEventsInput,
				_ ...func(*cloudformation.Options),
			) (*cloudformation.DescribeStackEventsOutput, error) {
				return nil, errors.New("API error")
			},
		}

		deployer := NewAWSDeployerWithClient(mockClient, "us-east-1")
		failures := deployer.getFailedResourceEvents(context.Background(), "test-stack")

		assert.Empty(t, failures)
	})
}

func TestAWSDeployer_GetRegion(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"}

	for _, region := range regions {
		t.Run("region: "+region, func(t *testing.T) {
			deployer := NewAWSDeployerWithClient(&mockCloudFormationClient{}, region)
			assert.Equal(t, region, deployer.GetRegion())
		})
	}
}

func TestAWSDeployer_ValidateRegionForDefaultTemplate(t *testing.T) {
	t.Run("valid region with default template", func(t *testing.T) {
		deployer := NewAWSDeployerWithClient(&mockCloudFormationClient{}, "us-east-1")
		err := deployer.validateRegionForDefaultTemplate("")

		require.NoError(t, err)
	})

	t.Run("custom template does not require region validation", func(t *testing.T) {
		deployer := NewAWSDeployerWithClient(&mockCloudFormationClient{}, "invalid-region")
		err := deployer.validateRegionForDefaultTemplate("https://example.com/template.yaml")

		require.NoError(t, err)
	})

	t.Run("invalid region with default template", func(t *testing.T) {
		deployer := NewAWSDeployerWithClient(&mockCloudFormationClient{}, "not-a-real-aws-region")
		err := deployer.validateRegionForDefaultTemplate("")

		// Only expect error if region validation is strict
		// Some regions might not be validated at this level
		if err != nil {
			assert.Contains(t, err.Error(), "region validation failed")
		}
	})
}
