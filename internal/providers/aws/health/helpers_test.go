package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager_buildRoleARNs(t *testing.T) {
	tests := []struct {
		name                   string
		cfg                    *Config
		taskRoleName           *string
		taskExecutionRoleName  *string
		expectedTaskRoleARN    string
		expectedTaskExecRoleARN string
	}{
		{
			name: "both roles provided",
			cfg: &Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          stringPtr("custom-task-role"),
			taskExecutionRoleName: stringPtr("custom-exec-role"),
			expectedTaskRoleARN:   "arn:aws:iam::123456789012:role/custom-task-role",
			expectedTaskExecRoleARN: "arn:aws:iam::123456789012:role/custom-exec-role",
		},
		{
			name: "only task role provided",
			cfg: &Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          stringPtr("custom-task-role"),
			taskExecutionRoleName: nil,
			expectedTaskRoleARN:   "arn:aws:iam::123456789012:role/custom-task-role",
			expectedTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
		},
		{
			name: "only exec role provided",
			cfg: &Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          nil,
			taskExecutionRoleName: stringPtr("custom-exec-role"),
			expectedTaskRoleARN:   "arn:aws:iam::123456789012:role/default-task",
			expectedTaskExecRoleARN: "arn:aws:iam::123456789012:role/custom-exec-role",
		},
		{
			name: "no custom roles, use defaults",
			cfg: &Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          nil,
			taskExecutionRoleName: nil,
			expectedTaskRoleARN:   "arn:aws:iam::123456789012:role/default-task",
			expectedTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
		},
		{
			name: "no default task role, custom provided",
			cfg: &Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          stringPtr("custom-task-role"),
			taskExecutionRoleName: nil,
			expectedTaskRoleARN:   "arn:aws:iam::123456789012:role/custom-task-role",
			expectedTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
		},
		{
			name: "no default task role, no custom provided",
			cfg: &Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          nil,
			taskExecutionRoleName: nil,
			expectedTaskRoleARN:   "",
			expectedTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
		},
		{
			name: "empty string roles treated as nil",
			cfg: &Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          stringPtr(""),
			taskExecutionRoleName: stringPtr(""),
			expectedTaskRoleARN:   "arn:aws:iam::123456789012:role/default-task",
			expectedTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{cfg: tt.cfg}
			taskRoleARN, taskExecRoleARN := m.buildRoleARNs(tt.taskRoleName, tt.taskExecutionRoleName)

			assert.Equal(t, tt.expectedTaskRoleARN, taskRoleARN)
			assert.Equal(t, tt.expectedTaskExecRoleARN, taskExecRoleARN)
		})
	}
}

func TestExtractRoleNameFromARN(t *testing.T) {
	tests := []struct {
		name     string
		arn      string
		expected string
	}{
		{
			name:     "standard IAM role ARN",
			arn:      "arn:aws:iam::123456789012:role/my-role",
			expected: "my-role",
		},
		{
			name:     "role with path",
			arn:      "arn:aws:iam::123456789012:role/path/to/my-role",
			expected: "my-role",
		},
		{
			name:     "role with multiple path segments",
			arn:      "arn:aws:iam::123456789012:role/application/production/my-role",
			expected: "my-role",
		},
		{
			name:     "simple role name without path",
			arn:      "arn:aws:iam::123456789012:role/role-name",
			expected: "role-name",
		},
		{
			name:     "not an ARN format",
			arn:      "not-an-arn",
			expected: "not-an-arn",
		},
		{
			name:     "empty string",
			arn:      "",
			expected: "",
		},
		{
			name:     "ARN without slash",
			arn:      "arn:aws:iam::123456789012:role",
			expected: "arn:aws:iam::123456789012:role",
		},
		{
			name:     "empty string after split edge case",
			arn:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRoleNameFromARN(tt.arn)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
