// Package orchestrator provides AWS-specific implementations for runvoy orchestrator.
// This file contains shared tagging utilities for AWS resources.
package orchestrator

import (
	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"runvoy/internal/providers/aws/secrets"
)

// GetStandardECSTags returns the standard tags in ECS tag format.
func GetStandardECSTags() []ecsTypes.Tag {
	standardTags := secrets.GetStandardTags()
	tags := make([]ecsTypes.Tag, len(standardTags))
	for i, tag := range standardTags {
		tags[i] = ecsTypes.Tag{
			Key:   awsStd.String(tag.Key),
			Value: awsStd.String(tag.Value),
		}
	}
	return tags
}
