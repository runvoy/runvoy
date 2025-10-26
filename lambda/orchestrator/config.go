package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

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
