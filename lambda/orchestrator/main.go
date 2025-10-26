package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"golang.org/x/crypto/bcrypt"
)

type ExecutionRequest struct {
	ExecutionID    string            `json:"execution_id"`
	Repo           string            `json:"repo"`
	Branch         string            `json:"branch,omitempty"`
	Command        string            `json:"command"`
	Image          string            `json:"image,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

type Request struct {
	Action      string            `json:"action"`
	TaskArn     string            `json:"task_arn,omitempty"`
	ExecutionID string            `json:"execution_id,omitempty"`
	
	// Execution parameters (for "exec" action)
	Repo           string            `json:"repo,omitempty"`
	Branch         string            `json:"branch,omitempty"`
	Command        string            `json:"command,omitempty"`
	Image          string            `json:"image,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

type Response struct {
	ExecutionID   string `json:"execution_id,omitempty"`
	TaskArn       string `json:"task_arn,omitempty"`
	Status        string `json:"status,omitempty"`
	DesiredStatus string `json:"desired_status,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	LogStream     string `json:"log_stream,omitempty"`
	Logs          string `json:"logs,omitempty"`
	Error         string `json:"error,omitempty"`
}

var (
	cfg        aws.Config
	ecsClient  *ecs.Client
	logsClient *cloudwatchlogs.Client

	apiKeyHash    string
	githubToken   string
	gitlabToken   string
	sshPrivateKey string
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

	ecsClient = ecs.NewFromConfig(cfg)
	logsClient = cloudwatchlogs.NewFromConfig(cfg)

	apiKeyHash = os.Getenv("API_KEY_HASH")
	githubToken = os.Getenv("GITHUB_TOKEN")
	gitlabToken = os.Getenv("GITLAB_TOKEN")
	sshPrivateKey = os.Getenv("SSH_PRIVATE_KEY")
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
	// Validate required fields
	if req.Repo == "" {
		return Response{}, fmt.Errorf("repo required")
	}
	if req.Command == "" {
		return Response{}, fmt.Errorf("command required")
	}

	// Generate execution ID if not provided
	execID := req.ExecutionID
	if execID == "" {
		execID = generateExecutionID()
	}

	// Set defaults
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}

	image := req.Image
	if image == "" {
		// Use a generic base image with git installed
		// Users are encouraged to specify their own image (terraform, python, etc.)
		image = "ubuntu:22.04"
	}

	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = 1800 // 30 minutes default
	}

	// Construct the shell command that will:
	// 1. Setup git credentials (if provided)
	// 2. Clone the repository
	// 3. Change to the repo directory
	// 4. Execute the user's command
	shellCommand := buildShellCommand(req.Repo, branch, req.Command, githubToken, gitlabToken, sshPrivateKey)

	// Build environment variables for the container
	// Include user-provided environment variables
	envVars := []ecsTypes.KeyValuePair{
		{Name: aws.String("EXECUTION_ID"), Value: aws.String(execID)},
	}

	// Add user-provided environment variables
	for key, value := range req.Env {
		envVars = append(envVars, ecsTypes.KeyValuePair{
			Name:  aws.String(key),
			Value: aws.String(value),
		})
	}

	// Run Fargate task
	runTaskInput := &ecs.RunTaskInput{
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
		Tags: []ecsTypes.Tag{
			{Key: aws.String("ExecutionID"), Value: aws.String(execID)},
			{Key: aws.String("Repo"), Value: aws.String(req.Repo)},
		},
	}

	// Override container with our command, image, and environment
	containerOverride := ecsTypes.ContainerOverride{
		Name:        aws.String("executor"),
		Image:       aws.String(image),
		Environment: envVars,
		// Override the command to run our shell script
		Command: []string{"/bin/sh", "-c", shellCommand},
	}

	runTaskInput.Overrides = &ecsTypes.TaskOverride{
		ContainerOverrides: []ecsTypes.ContainerOverride{containerOverride},
	}

	runTaskResp, err := ecsClient.RunTask(ctx, runTaskInput)
	if err != nil {
		return Response{}, fmt.Errorf("failed to run task: %v", err)
	}

	if len(runTaskResp.Tasks) == 0 {
		return Response{}, fmt.Errorf("no task created")
	}

	task := runTaskResp.Tasks[0]
	taskArn := *task.TaskArn

	// Generate log stream name (ECS uses task ID)
	taskID := taskArn[len(taskArn)-36:] // Last 36 chars (UUID)
	logStream := fmt.Sprintf("task/%s", taskID)

	return Response{
		ExecutionID: execID,
		TaskArn:     taskArn,
		Status:      "starting",
		LogStream:   logStream,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
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
		Status:        aws.ToString(task.LastStatus),
		DesiredStatus: aws.ToString(task.DesiredStatus),
		CreatedAt:     createdAt,
	}, nil
}

func handleLogs(ctx context.Context, req Request) (Response, error) {
	if req.ExecutionID == "" {
		return Response{}, fmt.Errorf("execution_id required")
	}

	// Try to find log streams for this execution
	// ECS creates log streams with format: task/<task-id>
	// We stored execution ID as a tag, but for simplicity, we'll search by prefix
	
	streamPrefix := "task/"
	
	filterResp, err := logsClient.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        &logGroup,
		LogStreamNamePrefix: &streamPrefix,
		Limit:               aws.Int32(1000),
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to get logs: %v", err)
	}

	var logs string
	for _, event := range filterResp.Events {
		if event.Message != nil {
			logs += *event.Message + "\n"
		}
	}

	if logs == "" {
		logs = "No logs available yet. The task may still be starting."
	}

	return Response{
		Logs: logs,
	}, nil
}

func generateExecutionID() string {
	return fmt.Sprintf("exec_%s_%06d", 
		time.Now().UTC().Format("20060102_150405"),
		time.Now().Nanosecond()/1000)
}

// buildShellCommand constructs a shell script that:
// 1. Installs git if not present (for generic images)
// 2. Configures git credentials
// 3. Clones the repository
// 4. Executes the user's command
//
// NOTE: This is a pragmatic bash solution for MVP.
// Future: Consider more robust solutions (Go binary, Python script, etc.)
func buildShellCommand(repo, branch, userCommand, githubToken, gitlabToken, sshKey string) string {
	script := `set -e
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "mycli Remote Execution"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
`

	// Install git if not present (for minimal images like alpine, ubuntu)
	script += `
if ! command -v git &> /dev/null; then
  echo "→ Installing git..."
  if command -v apk &> /dev/null; then
    apk add --no-cache git openssh-client
  elif command -v apt-get &> /dev/null; then
    apt-get update && apt-get install -y git openssh-client
  elif command -v yum &> /dev/null; then
    yum install -y git openssh-clients
  else
    echo "ERROR: Cannot install git - unsupported package manager"
    exit 1
  fi
fi
`

	// Setup git credentials
	if githubToken != "" {
		script += fmt.Sprintf(`
echo "→ Configuring GitHub authentication..."
git config --global credential.helper store
echo "https://%s:x-oauth-basic@github.com" > ~/.git-credentials
chmod 600 ~/.git-credentials
`, githubToken)
	} else if gitlabToken != "" {
		script += fmt.Sprintf(`
echo "→ Configuring GitLab authentication..."
git config --global credential.helper store
echo "https://oauth2:%s@gitlab.com" > ~/.git-credentials
chmod 600 ~/.git-credentials
`, gitlabToken)
	} else if sshKey != "" {
		script += fmt.Sprintf(`
echo "→ Configuring SSH authentication..."
mkdir -p ~/.ssh
echo "%s" | base64 -d > ~/.ssh/id_rsa
chmod 600 ~/.ssh/id_rsa
ssh-keyscan github.com >> ~/.ssh/known_hosts 2>/dev/null
ssh-keyscan gitlab.com >> ~/.ssh/known_hosts 2>/dev/null
ssh-keyscan bitbucket.org >> ~/.ssh/known_hosts 2>/dev/null
`, sshKey)
	}

	// Clone repository
	script += fmt.Sprintf(`
echo "→ Repository: %s"
echo "→ Branch: %s"
echo "→ Cloning repository..."
git clone --depth 1 --branch "%s" "%s" /workspace/repo || {
  echo "ERROR: Failed to clone repository"
  echo "Please verify:"
  echo "  - Repository URL is correct"
  echo "  - Branch '%s' exists"
  echo "  - Git credentials are configured (for private repos)"
  exit 1
}
cd /workspace/repo
echo "✓ Repository cloned"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Executing command: %s"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
`, repo, branch, branch, repo, branch, userCommand)

	// Execute user command
	script += userCommand + "\n"

	// Capture exit code and cleanup
	script += `
EXIT_CODE=$?
echo ""
if [ $EXIT_CODE -eq 0 ]; then
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "✓ Command completed successfully"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
else
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "✗ Command failed with exit code: $EXIT_CODE"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi
rm -f ~/.git-credentials ~/.ssh/id_rsa
exit $EXIT_CODE
`

	return script
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
