package health

import (
	"context"
	"errors"

	"runvoy/internal/api"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmTypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type mockIAMClient struct {
	getRoleFunc func(
		ctx context.Context,
		params *iam.GetRoleInput,
		optFns ...func(*iam.Options),
	) (*iam.GetRoleOutput, error)
}

func (m *mockIAMClient) GetRole(
	ctx context.Context,
	params *iam.GetRoleInput,
	optFns ...func(*iam.Options),
) (*iam.GetRoleOutput, error) {
	if m.getRoleFunc != nil {
		return m.getRoleFunc(ctx, params, optFns...)
	}
	return &iam.GetRoleOutput{}, nil
}

type mockImageRepo struct {
	images []api.ImageInfo
}

func (m *mockImageRepo) ListImages(_ context.Context) ([]api.ImageInfo, error) {
	return m.images, nil
}

type mockSSMClient struct {
	getParameterFunc func(
		ctx context.Context,
		params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.GetParameterOutput, error)
	listTagsForResourceFunc func(
		ctx context.Context,
		params *ssm.ListTagsForResourceInput,
		optFns ...func(*ssm.Options),
	) (*ssm.ListTagsForResourceOutput, error)
	addTagsToResourceFunc func(
		ctx context.Context,
		params *ssm.AddTagsToResourceInput,
		optFns ...func(*ssm.Options),
	) (*ssm.AddTagsToResourceOutput, error)
	describeParametersFunc func(
		ctx context.Context,
		params *ssm.DescribeParametersInput,
		optFns ...func(*ssm.Options),
	) (*ssm.DescribeParametersOutput, error)
	deleteParameterFunc func(
		ctx context.Context,
		params *ssm.DeleteParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.DeleteParameterOutput, error)
	putParameterFunc func(
		ctx context.Context,
		params *ssm.PutParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.PutParameterOutput, error)
}

func (m *mockSSMClient) GetParameter(
	ctx context.Context,
	params *ssm.GetParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.GetParameterOutput, error) {
	if m.getParameterFunc != nil {
		return m.getParameterFunc(ctx, params, optFns...)
	}
	return &ssm.GetParameterOutput{}, nil
}

func (m *mockSSMClient) ListTagsForResource(
	ctx context.Context,
	params *ssm.ListTagsForResourceInput,
	optFns ...func(*ssm.Options),
) (*ssm.ListTagsForResourceOutput, error) {
	if m.listTagsForResourceFunc != nil {
		return m.listTagsForResourceFunc(ctx, params, optFns...)
	}
	return &ssm.ListTagsForResourceOutput{}, nil
}

func (m *mockSSMClient) AddTagsToResource(
	ctx context.Context,
	params *ssm.AddTagsToResourceInput,
	optFns ...func(*ssm.Options),
) (*ssm.AddTagsToResourceOutput, error) {
	if m.addTagsToResourceFunc != nil {
		return m.addTagsToResourceFunc(ctx, params, optFns...)
	}
	return &ssm.AddTagsToResourceOutput{}, nil
}

func (m *mockSSMClient) DescribeParameters(
	ctx context.Context,
	params *ssm.DescribeParametersInput,
	optFns ...func(*ssm.Options),
) (*ssm.DescribeParametersOutput, error) {
	if m.describeParametersFunc != nil {
		return m.describeParametersFunc(ctx, params, optFns...)
	}
	return &ssm.DescribeParametersOutput{Parameters: []ssmTypes.ParameterMetadata{}}, nil
}

func (m *mockSSMClient) DeleteParameter(
	ctx context.Context,
	params *ssm.DeleteParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.DeleteParameterOutput, error) {
	if m.deleteParameterFunc != nil {
		return m.deleteParameterFunc(ctx, params, optFns...)
	}
	return &ssm.DeleteParameterOutput{}, nil
}

func (m *mockSSMClient) PutParameter(
	ctx context.Context,
	params *ssm.PutParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.PutParameterOutput, error) {
	if m.putParameterFunc != nil {
		return m.putParameterFunc(ctx, params, optFns...)
	}
	return &ssm.PutParameterOutput{}, nil
}

type mockECSClient struct {
	listTaskDefinitionsFunc func(
		ctx context.Context,
		params *ecs.ListTaskDefinitionsInput,
		optFns ...func(*ecs.Options),
	) (*ecs.ListTaskDefinitionsOutput, error)
	listTagsForResourceFunc func(
		ctx context.Context,
		params *ecs.ListTagsForResourceInput,
		optFns ...func(*ecs.Options),
	) (*ecs.ListTagsForResourceOutput, error)
}

func (m *mockECSClient) RunTask(
	_ context.Context,
	_ *ecs.RunTaskInput,
	_ ...func(*ecs.Options),
) (*ecs.RunTaskOutput, error) {
	return nil, errors.New("not implemented")
}

func (m *mockECSClient) TagResource(
	_ context.Context,
	_ *ecs.TagResourceInput,
	_ ...func(*ecs.Options),
) (*ecs.TagResourceOutput, error) {
	return nil, errors.New("not implemented")
}

func (m *mockECSClient) ListTasks(
	_ context.Context,
	_ *ecs.ListTasksInput,
	_ ...func(*ecs.Options),
) (*ecs.ListTasksOutput, error) {
	return nil, errors.New("not implemented")
}

func (m *mockECSClient) DescribeTasks(
	_ context.Context,
	_ *ecs.DescribeTasksInput,
	_ ...func(*ecs.Options),
) (*ecs.DescribeTasksOutput, error) {
	return nil, errors.New("not implemented")
}

func (m *mockECSClient) StopTask(
	_ context.Context,
	_ *ecs.StopTaskInput,
	_ ...func(*ecs.Options),
) (*ecs.StopTaskOutput, error) {
	return nil, errors.New("not implemented")
}

func (m *mockECSClient) DescribeTaskDefinition(
	_ context.Context,
	_ *ecs.DescribeTaskDefinitionInput,
	_ ...func(*ecs.Options),
) (*ecs.DescribeTaskDefinitionOutput, error) {
	return nil, errors.New("not implemented")
}

func (m *mockECSClient) ListTagsForResource(
	ctx context.Context,
	params *ecs.ListTagsForResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTagsForResourceOutput, error) {
	if m.listTagsForResourceFunc != nil {
		return m.listTagsForResourceFunc(ctx, params, optFns...)
	}
	return &ecs.ListTagsForResourceOutput{}, nil
}

func (m *mockECSClient) ListTaskDefinitions(
	ctx context.Context,
	params *ecs.ListTaskDefinitionsInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTaskDefinitionsOutput, error) {
	if m.listTaskDefinitionsFunc != nil {
		return m.listTaskDefinitionsFunc(ctx, params, optFns...)
	}
	return &ecs.ListTaskDefinitionsOutput{}, nil
}

func (m *mockECSClient) RegisterTaskDefinition(
	_ context.Context,
	_ *ecs.RegisterTaskDefinitionInput,
	_ ...func(*ecs.Options),
) (*ecs.RegisterTaskDefinitionOutput, error) {
	return nil, errors.New("not implemented")
}

func (m *mockECSClient) DeregisterTaskDefinition(
	_ context.Context,
	_ *ecs.DeregisterTaskDefinitionInput,
	_ ...func(*ecs.Options),
) (*ecs.DeregisterTaskDefinitionOutput, error) {
	return nil, errors.New("not implemented")
}

func (m *mockECSClient) DeleteTaskDefinitions(
	_ context.Context,
	_ *ecs.DeleteTaskDefinitionsInput,
	_ ...func(*ecs.Options),
) (*ecs.DeleteTaskDefinitionsOutput, error) {
	return nil, errors.New("not implemented")
}

func (m *mockECSClient) UntagResource(
	_ context.Context,
	_ *ecs.UntagResourceInput,
	_ ...func(*ecs.Options),
) (*ecs.UntagResourceOutput, error) {
	return nil, errors.New("not implemented")
}

func sampleImage(customTaskRole, customExecRole string) api.ImageInfo {
	return api.ImageInfo{
		ImageID:               "img-1",
		Image:                 "alpine:latest",
		TaskRoleName:          stringPtr(customTaskRole),
		TaskExecutionRoleName: stringPtr(customExecRole),
	}
}
