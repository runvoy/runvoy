package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var cfg *Config

func init() {
	var err error
	cfg, err = InitConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize config: %v", err))
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Authenticate
	apiKey := request.Headers["x-api-key"]
	if apiKey == "" {
		apiKey = request.Headers["X-Api-Key"] // Try capitalized version
	}

	user, err := authenticate(ctx, cfg, apiKey)
	if err != nil {
		return errorResponse(401, fmt.Sprintf("unauthorized: %v", err))
	}

	// Route based on HTTP method and path
	method := request.HTTPMethod
	path := request.Path

	fmt.Printf("[DEBUG] Routing: %s %s\n", method, path)

	var resp interface{}

	// Parse request body to check for action-based API (legacy support)
	var actionReq struct {
		Action string `json:"action"`
	}
	json.Unmarshal([]byte(request.Body), &actionReq)

	// Route based on action for legacy API, or path for REST API
	if actionReq.Action != "" {
		// Legacy action-based API
		switch actionReq.Action {
		case "exec":
			// Map legacy ExecRequest to CreateExecutionRequest
			var legacyReq struct {
				Action         string            `json:"action"`
				Repo           string            `json:"repo"`
				Branch         string            `json:"branch,omitempty"`
				Command        string            `json:"command"`
				Image          string            `json:"image,omitempty"`
				Env            map[string]string `json:"env,omitempty"`
				TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
				SkipGit        bool              `json:"skip_git,omitempty"`
			}
			if err := json.Unmarshal([]byte(request.Body), &legacyReq); err != nil {
				return errorResponse(400, fmt.Sprintf("invalid request: %v", err))
			}

			// Build the command: git clone if Repo is specified, otherwise just run the command
			finalCommand := legacyReq.Command
			if legacyReq.Repo != "" && !legacyReq.SkipGit {
				// Build git clone command
				branch := legacyReq.Branch
				if branch == "" {
					branch = "main"
				}
				// Escape the repo URL and command for shell safety
				repoEscaped := shellEscape(legacyReq.Repo)
				branchEscaped := shellEscape(branch)
				cmdEscaped := shellEscape(legacyReq.Command)

				// Wrap in shell script structure
				finalCommand = fmt.Sprintf(`set -e
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "mycli Remote Execution"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "→ Repository: %s"
echo "→ Branch: %s"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Cloning repository..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
git clone %s repo
cd repo
git checkout %s
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Executing command..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
eval %s
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
exit $EXIT_CODE`, legacyReq.Repo, branch, repoEscaped, branchEscaped, cmdEscaped)
			}

			// Convert to CreateExecutionRequest format
			createReq := CreateExecutionRequest{
				Command:        finalCommand,
				Image:          legacyReq.Image,
				Env:            legacyReq.Env,
				TimeoutSeconds: legacyReq.TimeoutSeconds,
			}

			// Create a modified request with the converted body
			modifiedRequest := request
			bodyBytes, _ := json.Marshal(createReq)
			modifiedRequest.Body = string(bodyBytes)

			resp, err = handleCreateExecution(ctx, cfg, user, modifiedRequest)
		default:
			return errorResponse(400, fmt.Sprintf("unknown action: %s", actionReq.Action))
		}
	} else {
		// REST-style API
		switch {
		case method == "POST" && path == "/executions":
			resp, err = handleCreateExecution(ctx, cfg, user, request)
		case method == "GET" && path == "/executions":
			resp, err = handleListExecutions(ctx, cfg, user, request)
		case method == "GET" && len(path) > 12 && path[:12] == "/executions/":
			// Extract execution ID from path
			parts := splitPath(path)
			if len(parts) >= 2 {
				executionID := parts[1]
				if len(parts) == 3 && parts[2] == "logs" {
					resp, err = handleGetExecutionLogs(ctx, cfg, user, executionID)
				} else if len(parts) == 2 {
					resp, err = handleGetExecution(ctx, cfg, user, executionID)
				} else {
					return errorResponse(404, "not found")
				}
			} else {
				return errorResponse(404, "not found")
			}
		case method == "GET" && path == "/locks":
			resp, err = handleListLocks(ctx, cfg, user)
		default:
			return errorResponse(404, fmt.Sprintf("not found: %s %s", method, path))
		}
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

// splitPath splits a path into parts, removing empty strings
func splitPath(path string) []string {
	parts := []string{}
	current := ""
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(path[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func main() {
	lambda.Start(handler)
}
