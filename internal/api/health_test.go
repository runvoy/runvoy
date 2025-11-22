package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthReconcileResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal full response", func(t *testing.T) {
		resp := HealthReconcileResponse{
			Status: "completed",
			Report: &HealthReport{
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				ComputeStatus: ComputeHealthStatus{
					TotalResources:    10,
					VerifiedCount:     8,
					RecreatedCount:    1,
					TagUpdatedCount:   1,
					OrphanedCount:     0,
					OrphanedResources: []string{},
				},
				SecretsStatus: SecretsHealthStatus{
					TotalSecrets:       5,
					VerifiedCount:      4,
					TagUpdatedCount:    1,
					MissingCount:       0,
					OrphanedCount:      0,
					OrphanedParameters: []string{},
				},
				IdentityStatus: IdentityHealthStatus{
					DefaultRolesVerified: true,
					CustomRolesVerified:  3,
					CustomRolesTotal:     3,
					MissingRoles:         []string{},
				},
				Issues:          []HealthIssue{},
				ReconciledCount: 15,
				ErrorCount:      0,
			},
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled HealthReconcileResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.Status, unmarshaled.Status)
		require.NotNil(t, unmarshaled.Report)
		assert.Equal(t, resp.Report.Timestamp, unmarshaled.Report.Timestamp)
		assert.Equal(t, resp.Report.ReconciledCount, unmarshaled.Report.ReconciledCount)
		assert.Equal(t, resp.Report.ErrorCount, unmarshaled.Report.ErrorCount)
	})

	t.Run("marshal response with nil report", func(t *testing.T) {
		resp := HealthReconcileResponse{
			Status: "pending",
			Report: nil,
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled HealthReconcileResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, "pending", unmarshaled.Status)
		assert.Nil(t, unmarshaled.Report)
	})
}

func TestHealthReconcileComputeStatusJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		status := ComputeHealthStatus{
			TotalResources:    100,
			VerifiedCount:     90,
			RecreatedCount:    5,
			TagUpdatedCount:   3,
			OrphanedCount:     2,
			OrphanedResources: []string{"ecs-task-1", "ecs-task-2"},
		}

		data, err := json.Marshal(status)
		require.NoError(t, err)

		var unmarshaled ComputeHealthStatus
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, status.TotalResources, unmarshaled.TotalResources)
		assert.Equal(t, status.VerifiedCount, unmarshaled.VerifiedCount)
		assert.Equal(t, status.RecreatedCount, unmarshaled.RecreatedCount)
		assert.Equal(t, status.TagUpdatedCount, unmarshaled.TagUpdatedCount)
		assert.Equal(t, status.OrphanedCount, unmarshaled.OrphanedCount)
		assert.Equal(t, status.OrphanedResources, unmarshaled.OrphanedResources)
	})

	t.Run("json field names", func(t *testing.T) {
		status := ComputeHealthStatus{
			TotalResources:    10,
			OrphanedResources: []string{"resource-1"},
		}

		data, err := json.Marshal(status)
		require.NoError(t, err)

		jsonStr := string(data)
		assert.Contains(t, jsonStr, "total_resources")
		assert.Contains(t, jsonStr, "verified_count")
		assert.Contains(t, jsonStr, "recreated_count")
		assert.Contains(t, jsonStr, "orphaned_resources")
	})
}

func TestHealthReconcileSecretsStatusJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		status := SecretsHealthStatus{
			TotalSecrets:       20,
			VerifiedCount:      18,
			TagUpdatedCount:    1,
			MissingCount:       1,
			OrphanedCount:      0,
			OrphanedParameters: []string{},
		}

		data, err := json.Marshal(status)
		require.NoError(t, err)

		var unmarshaled SecretsHealthStatus
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, status.TotalSecrets, unmarshaled.TotalSecrets)
		assert.Equal(t, status.VerifiedCount, unmarshaled.VerifiedCount)
		assert.Equal(t, status.MissingCount, unmarshaled.MissingCount)
		assert.Equal(t, status.OrphanedParameters, unmarshaled.OrphanedParameters)
	})

	t.Run("json field names", func(t *testing.T) {
		status := SecretsHealthStatus{
			TotalSecrets:       5,
			OrphanedParameters: []string{"param-1"},
		}

		data, err := json.Marshal(status)
		require.NoError(t, err)

		jsonStr := string(data)
		assert.Contains(t, jsonStr, "total_secrets")
		assert.Contains(t, jsonStr, "orphaned_parameters")
	})
}

func TestHealthReconcileIdentityStatusJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		status := IdentityHealthStatus{
			DefaultRolesVerified: true,
			CustomRolesVerified:  5,
			CustomRolesTotal:     6,
			MissingRoles:         []string{"admin-role"},
		}

		data, err := json.Marshal(status)
		require.NoError(t, err)

		var unmarshaled IdentityHealthStatus
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, status.DefaultRolesVerified, unmarshaled.DefaultRolesVerified)
		assert.Equal(t, status.CustomRolesVerified, unmarshaled.CustomRolesVerified)
		assert.Equal(t, status.CustomRolesTotal, unmarshaled.CustomRolesTotal)
		assert.Equal(t, status.MissingRoles, unmarshaled.MissingRoles)
	})

	t.Run("json field names", func(t *testing.T) {
		status := IdentityHealthStatus{
			DefaultRolesVerified: false,
			MissingRoles:         []string{"role-1"},
		}

		data, err := json.Marshal(status)
		require.NoError(t, err)

		jsonStr := string(data)
		assert.Contains(t, jsonStr, "default_roles_verified")
		assert.Contains(t, jsonStr, "custom_roles_verified")
		assert.Contains(t, jsonStr, "missing_roles")
	})
}

func TestHealthReconcileIssueJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		issue := HealthIssue{
			ResourceType: "ECS_TASK",
			ResourceID:   "task-12345",
			Severity:     "warning",
			Message:      "Task definition outdated",
			Action:       "recreated",
		}

		data, err := json.Marshal(issue)
		require.NoError(t, err)

		var unmarshaled HealthIssue
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, issue.ResourceType, unmarshaled.ResourceType)
		assert.Equal(t, issue.ResourceID, unmarshaled.ResourceID)
		assert.Equal(t, issue.Severity, unmarshaled.Severity)
		assert.Equal(t, issue.Message, unmarshaled.Message)
		assert.Equal(t, issue.Action, unmarshaled.Action)
	})

	t.Run("json field names", func(t *testing.T) {
		issue := HealthIssue{
			ResourceType: "SECRET",
			ResourceID:   "secret-1",
			Severity:     "error",
			Message:      "test",
			Action:       "none",
		}

		data, err := json.Marshal(issue)
		require.NoError(t, err)

		jsonStr := string(data)
		assert.Contains(t, jsonStr, "resource_type")
		assert.Contains(t, jsonStr, "resource_id")
		assert.Contains(t, jsonStr, "severity")
		assert.Contains(t, jsonStr, "message")
		assert.Contains(t, jsonStr, "action")
	})
}

func TestHealthReportJSON(t *testing.T) {
	t.Run("marshal and unmarshal with issues", func(t *testing.T) {
		report := HealthReport{
			Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			ComputeStatus: ComputeHealthStatus{
				TotalResources: 5,
				VerifiedCount:  4,
			},
			SecretsStatus: SecretsHealthStatus{
				TotalSecrets:  3,
				VerifiedCount: 3,
			},
			IdentityStatus: IdentityHealthStatus{
				DefaultRolesVerified: true,
			},
			Issues: []HealthIssue{
				{
					ResourceType: "COMPUTE",
					ResourceID:   "task-1",
					Severity:     "warning",
					Message:      "Task needs recreation",
					Action:       "recreated",
				},
			},
			ReconciledCount: 8,
			ErrorCount:      1,
		}

		data, err := json.Marshal(report)
		require.NoError(t, err)

		var unmarshaled HealthReport
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, report.Timestamp, unmarshaled.Timestamp)
		assert.Equal(t, report.ReconciledCount, unmarshaled.ReconciledCount)
		assert.Equal(t, report.ErrorCount, unmarshaled.ErrorCount)
		require.Len(t, unmarshaled.Issues, 1)
		assert.Equal(t, "COMPUTE", unmarshaled.Issues[0].ResourceType)
	})
}
