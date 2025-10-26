package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/crypto/bcrypt"
)

type Request struct {
	Action      string `json:"action"`
	Command     string `json:"command,omitempty"`
	TaskArn     string `json:"task_arn,omitempty"`
	ExecutionID string `json:"execution_id,omitempty"`
}

type Response struct {
	ExecutionID   string `json:"execution_id,omitempty"`
	TaskArn       string `json:"task_arn,omitempty"`
	Status        string `json:"status,omitempty"`
	DesiredStatus string `json:"desired_status,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	Logs          string `json:"logs,omitempty"`
	Error         string `json:"error,omitempty"`
}

var (
	cfg        aws.Config
	s3Client   *s3.Client
	ecsClient  *ecs.Client
	logsClient *cloudwatchlogs.Client

	apiKeyHash    string
	codeBucket    string
	ecsCluster    string
	taskDef       string
	subnet1       string
	subnet2       string
	securityGroup string
	logGroup      string
)

func init() {
	var err error
	cfg, err = config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("failed to load AWS config: %v", err))
	}

	s3Client = s3.NewFromConfig(cfg)
	ecsClient = ecs.NewFromConfig(cfg)
	logsClient = cloudwatchlogs.NewFromConfig(cfg)

	apiKeyHash = os.Getenv("API_KEY_HASH")
	codeBucket = os.Getenv("CODE_BUCKET")
	ecsCluster = os.Getenv("ECS_CLUSTER")
	taskDef = os.Getenv("TASK_DEFINITION")
	subnet1 = os.Getenv("SUBNET_1")
	subnet2 = os.Getenv("SUBNET_2")
	securityGroup = os.Getenv("SECURITY_GROUP")
	logGroup = os.Getenv("LOG_GROUP")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Parse request body
	var req Request
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(400, fmt.Sprintf("invalid request: %v", err))
	}

	// Authenticate
	apiKey := request.Headers["x-api-key"]
	if apiKey == "" {
		apiKey = request.Headers["X-Api-Key"] // Try capitalized version
	}

	if !authenticate(apiKey) {
		return errorResponse(401, "unauthorized")
	}

	// Route to handler
	var resp Response
	var err error

	switch req.Action {
	case "exec":
		resp, err = handleExec(ctx, req)
	case "status":
		resp, err = handleStatus(ctx, req)
	case "logs":
		resp, err = handleLogs(ctx, req)
	default:
		return errorResponse(400, "invalid action")
	}

	if err != nil {
		return errorResponse(500, err.Error())
	}

	body, _ := json.Marshal(resp)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func authenticate(apiKey string) bool {
	if apiKey == "" || apiKeyHash == "" {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(apiKeyHash), []byte(apiKey))
	return err == nil
}

func handleExec(ctx context.Context, req Request) (Response, error) {
	if req.Command == "" {
		return Response{}, fmt.Errorf("command required")
	}

	// Generate execution ID
	execID := time.Now().UTC().Format("20060102-150405-") + fmt.Sprintf("%06d", time.Now().Nanosecond()/1000)

	// Store command in S3
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &codeBucket,
		Key:    aws.String(fmt.Sprintf("commands/%s", execID)),
		Body:   strings.NewReader(req.Command),
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to store command: %v", err)
	}

	// Run Fargate task
	runTaskResp, err := ecsClient.RunTask(ctx, &ecs.RunTaskInput{
		Cluster:        &ecsCluster,
		TaskDefinition: &taskDef,
		LaunchType:     ecsTypes.LaunchTypeFargate,
		NetworkConfiguration: &ecsTypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
				Subnets:        []string{subnet1, subnet2},
				SecurityGroups: []string{securityGroup},
				AssignPublicIp: ecsTypes.AssignPublicIpEnabled,
			},
		},
		Overrides: &ecsTypes.TaskOverride{
			ContainerOverrides: []ecsTypes.ContainerOverride{
				{
					Name: aws.String("executor"),
					Command: []string{
						"/bin/sh",
						"-c",
						fmt.Sprintf("aws s3 cp s3://%s/commands/%s /tmp/cmd && sh /tmp/cmd", codeBucket, execID),
					},
				},
			},
		},
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to run task: %v", err)
	}

	if len(runTaskResp.Tasks) == 0 {
		return Response{}, fmt.Errorf("no task created")
	}

	taskArn := *runTaskResp.Tasks[0].TaskArn

	return Response{
		ExecutionID: execID,
		TaskArn:     taskArn,
	}, nil
}

func handleStatus(ctx context.Context, req Request) (Response, error) {
	if req.TaskArn == "" {
		return Response{}, fmt.Errorf("task_arn required")
	}

	describeResp, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &ecsCluster,
		Tasks:   []string{req.TaskArn},
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to describe task: %v", err)
	}

	if len(describeResp.Tasks) == 0 {
		return Response{}, fmt.Errorf("task not found")
	}

	task := describeResp.Tasks[0]
	createdAt := ""
	if task.CreatedAt != nil {
		createdAt = task.CreatedAt.Format(time.RFC3339)
	}

	return Response{
		Status:        *task.LastStatus,
		DesiredStatus: aws.ToString(task.DesiredStatus),
		CreatedAt:     createdAt,
	}, nil
}

func handleLogs(ctx context.Context, req Request) (Response, error) {
	if req.ExecutionID == "" {
		return Response{}, fmt.Errorf("execution_id required")
	}

	streamPrefix := fmt.Sprintf("task/%s", req.ExecutionID)

	filterResp, err := logsClient.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        &logGroup,
		LogStreamNamePrefix: &streamPrefix,
		Limit:               aws.Int32(100),
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to get logs: %v", err)
	}

	var logs []string
	for _, event := range filterResp.Events {
		if event.Message != nil {
			logs = append(logs, *event.Message)
		}
	}

	logsOutput := ""
	if len(logs) > 0 {
		logsOutput = logs[0]
		for i := 1; i < len(logs); i++ {
			logsOutput += "\n" + logs[i]
		}
	}

	return Response{
		Logs: logsOutput,
	}, nil
}

func errorResponse(statusCode int, message string) (events.APIGatewayProxyResponse, error) {
	resp := Response{Error: message}
	body, _ := json.Marshal(resp)

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func main() {
	lambda.Start(handler)
}
