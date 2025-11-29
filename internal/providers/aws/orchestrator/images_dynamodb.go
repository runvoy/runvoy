package orchestrator

import (
	"fmt"
	"regexp"
	"strings"
)

// buildRoleARN constructs a full IAM role ARN from a role name and account ID.
// Returns an empty string if roleName is nil or empty.
func buildRoleARN(roleName *string, accountID, _ string) string {
	if roleName == nil || *roleName == "" {
		return ""
	}
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, *roleName)
}

// sanitizeImageIDForTaskDef sanitizes an ImageID for use as an ECS task definition family name.
// ECS task definition family names must match [a-zA-Z0-9_-]+ (no dots or other special chars).
// Replaces invalid characters (dots, etc.) with hyphens.
func sanitizeImageIDForTaskDef(imageID string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := re.ReplaceAllString(imageID, "-")
	re2 := regexp.MustCompile(`-+`)
	sanitized = re2.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	return "runvoy-" + sanitized
}

// looksLikeImageID checks if a string looks like an ImageID format.
// ImageID format: {name}:{tag}-{8-char-hash}.
func looksLikeImageID(s string) bool {
	const hashLength = 8
	lastDashIdx := strings.LastIndex(s, "-")
	if lastDashIdx == -1 {
		return false
	}
	hashPart := s[lastDashIdx+1:]
	if len(hashPart) == hashLength {
		for _, c := range hashPart {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
				return false
			}
		}
		beforeHash := s[:lastDashIdx]
		return strings.Contains(beforeHash, ":")
	}
	return false
}
