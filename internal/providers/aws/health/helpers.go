package health

import (
	"fmt"
	"strings"
)

func (m *Manager) buildRoleARNs(taskRoleName, taskExecutionRoleName *string) (taskRoleARN, taskExecRoleARN string) {
	taskRoleARN = ""
	taskExecRoleARN = m.cfg.DefaultTaskExecRoleARN

	if taskRoleName != nil && *taskRoleName != "" {
		taskRoleARN = fmt.Sprintf("arn:aws:iam::%s:role/%s", m.cfg.AccountID, *taskRoleName)
	} else if m.cfg.DefaultTaskRoleARN != "" {
		taskRoleARN = m.cfg.DefaultTaskRoleARN
	}

	if taskExecutionRoleName != nil && *taskExecutionRoleName != "" {
		taskExecRoleARN = fmt.Sprintf("arn:aws:iam::%s:role/%s", m.cfg.AccountID, *taskExecutionRoleName)
	}

	return taskRoleARN, taskExecRoleARN
}

func extractRoleNameFromARN(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return arn
}
