package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		APIEndpoint: "https://example.com",
		APIKey:      "test-key",
	}
	logger := testutil.SilentLogger()

	c := New(cfg, logger)

	if c.config != cfg {
		t.Errorf("Expected config to be %v, got %v", cfg, c.config)
	}

	if c.logger != logger {
		t.Errorf("Expected logger to be %v, got %v", logger, c.logger)
	}
}

func TestClient_Do(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		request        Request
		wantErr        bool
		wantStatusCode int
		validateResp   func(t *testing.T, resp *Response)
	}{
		{
			name: "successful GET request",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/api/v1/test", r.URL.Path)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
					assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"message": "success"}`))
				}))
			},
			request: Request{
				Method: "GET",
				Path:   "/api/v1/test",
			},
			wantErr:        false,
			wantStatusCode: http.StatusOK,
			validateResp: func(t *testing.T, resp *Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Contains(t, string(resp.Body), "success")
			},
		},
		{
			name: "successful POST request with body",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					assert.Equal(t, "test-value", body["test"])
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"id": "123"}`))
				}))
			},
			request: Request{
				Method: "POST",
				Path:   "/api/v1/test",
				Body:   map[string]string{"test": "test-value"},
			},
			wantErr:        false,
			wantStatusCode: http.StatusCreated,
		},
		{
			name: "server error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error": "internal error"}`))
				}))
			},
			request: Request{
				Method: "GET",
				Path:   "/api/v1/test",
			},
			wantErr:        false,
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "invalid JSON body",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			request: Request{
				Method: "POST",
				Path:   "/api/v1/test",
				Body:   make(chan int), // cannot be marshaled to JSON
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			cfg := &config.Config{
				APIEndpoint: server.URL,
				APIKey:      "test-api-key",
			}
			c := New(cfg, testutil.SilentLogger())

			resp, err := c.Do(context.Background(), tt.request)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.wantStatusCode, resp.StatusCode)
				if tt.validateResp != nil {
					tt.validateResp(t, resp)
				}
			}
		})
	}
}

func TestClient_DoJSON(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		request     Request
		result      any
		wantErr     bool
		errContains string
	}{
		{
			name: "successful request with JSON response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"execution_id": "test-123", "status": "RUNNING"}`))
				}))
			},
			request: Request{
				Method: "GET",
				Path:   "/api/v1/executions/test-123",
			},
			result:  &api.ExecutionStatusResponse{},
			wantErr: false,
		},
		{
			name: "HTTP error response with error JSON",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"error": "Not Found", "details": "Resource not found"}`))
				}))
			},
			request: Request{
				Method: "GET",
				Path:   "/api/v1/executions/missing",
			},
			result:      &api.ExecutionStatusResponse{},
			wantErr:     true,
			errContains: "[404] Not Found: Resource not found",
		},
		{
			name: "HTTP error response with invalid JSON",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`not json`))
				}))
			},
			request: Request{
				Method: "GET",
				Path:   "/api/v1/test",
			},
			result:      &api.ExecutionStatusResponse{},
			wantErr:     true,
			errContains: "request failed with status 400",
		},
		{
			name: "invalid JSON in response body",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`not valid json`))
				}))
			},
			request: Request{
				Method: "GET",
				Path:   "/api/v1/test",
			},
			result:      &api.ExecutionStatusResponse{},
			wantErr:     true,
			errContains: "failed to parse response",
		},
		{
			name: "successful request with array response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[{"execution_id": "test-1"}, {"execution_id": "test-2"}]`))
				}))
			},
			request: Request{
				Method: "GET",
				Path:   "/api/v1/executions",
			},
			result:  &[]api.Execution{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			cfg := &config.Config{
				APIEndpoint: server.URL,
				APIKey:      "test-api-key",
			}
			c := New(cfg, testutil.SilentLogger())

			err := c.DoJSON(context.Background(), tt.request, tt.result)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClient_CreateUser(t *testing.T) {
	t.Run("successful user creation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/users/create", r.URL.Path)

			var req api.CreateUserRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "user@example.com", req.Email)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.CreateUserResponse{
				User: &api.User{
					Email:     "user@example.com",
					APIKey:    "test-key",
					CreatedAt: time.Now().UTC(),
				},
				ClaimToken: "claim-token-123",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.CreateUser(context.Background(), api.CreateUserRequest{
			Email: "user@example.com",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "user@example.com", resp.User.Email)
		assert.Equal(t, "claim-token-123", resp.ClaimToken)
	})

	t.Run("error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(api.ErrorResponse{
				Error:   "Conflict",
				Details: "User already exists",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.CreateUser(context.Background(), api.CreateUserRequest{
			Email: "user@example.com",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Conflict")
	})
}

func TestClient_RevokeUser(t *testing.T) {
	t.Run("successful user revocation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/users/revoke", r.URL.Path)

			var req api.RevokeUserRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "user@example.com", req.Email)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.RevokeUserResponse{
				Email:   "user@example.com",
				Message: "User revoked successfully",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.RevokeUser(context.Background(), api.RevokeUserRequest{
			Email: "user@example.com",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "user@example.com", resp.Email)
		assert.Equal(t, "User revoked successfully", resp.Message)
	})
}

func TestClient_ListUsers(t *testing.T) {
	t.Run("successful list users", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/users/", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.ListUsersResponse{
				Users: []*api.User{
					{Email: "user1@example.com"},
					{Email: "user2@example.com"},
				},
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.ListUsers(context.Background())

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Users, 2)
		assert.Equal(t, "user1@example.com", resp.Users[0].Email)
	})
}

func TestClient_GetHealth(t *testing.T) {
	t.Run("successful health check", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/health", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.HealthResponse{
				Status:   "healthy",
				Version:  "1.0.0",
				Provider: constants.AWS,
				Region:   "us-east-1",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.GetHealth(context.Background())

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "healthy", resp.Status)
		assert.Equal(t, "1.0.0", resp.Version)
		assert.Equal(t, constants.AWS, resp.Provider)
		assert.Equal(t, "us-east-1", resp.Region)
	})
}

func TestClient_RunCommand(t *testing.T) {
	t.Run("successful command execution", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/run", r.URL.Path)

			var req api.ExecutionRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "echo hello", req.Command)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.ExecutionResponse{
				ExecutionID: "exec-123",
				LogURL:      "https://example.com/logs/exec-123",
				Status:      "RUNNING",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.RunCommand(context.Background(), &api.ExecutionRequest{
			Command: "echo hello",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "exec-123", resp.ExecutionID)
		assert.Equal(t, "RUNNING", resp.Status)
	})
}

func TestClient_GetLogs(t *testing.T) {
	t.Run("successful log retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/executions/exec-123/logs", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.LogsResponse{
				ExecutionID: "exec-123",
				Status:      "RUNNING",
				Events: []api.LogEvent{
					{Timestamp: 1000, Message: "log line 1"},
					{Timestamp: 2000, Message: "log line 2"},
				},
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.GetLogs(context.Background(), "exec-123")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "exec-123", resp.ExecutionID)
		assert.Len(t, resp.Events, 2)
		assert.Equal(t, "log line 1", resp.Events[0].Message)
	})
}

func TestClient_GetExecutionStatus(t *testing.T) {
	t.Run("successful status retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.True(t, strings.HasPrefix(r.URL.Path, "/api/v1/executions/"))
			assert.True(t, strings.HasSuffix(r.URL.Path, "/status"))

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.ExecutionStatusResponse{
				ExecutionID: "exec-123",
				Status:      "SUCCEEDED",
				ExitCode:    intPtr(0),
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.GetExecutionStatus(context.Background(), "exec-123")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "exec-123", resp.ExecutionID)
		assert.Equal(t, "SUCCEEDED", resp.Status)
		assert.NotNil(t, resp.ExitCode)
		assert.Equal(t, 0, *resp.ExitCode)
	})
}

func TestClient_KillExecution(t *testing.T) {
	t.Run("successful execution kill", func(t *testing.T) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "DELETE", r.Method)
			assert.Equal(t, "/api/v1/executions/exec-123", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.KillExecutionResponse{
				ExecutionID: "exec-123",
				Message:     "Execution kill started successfully",
			})
		}
		server := httptest.NewServer(http.HandlerFunc(handler))
		defer server.Close()

		c := New(&config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}, testutil.SilentLogger())

		resp, err := c.KillExecution(context.Background(), "exec-123")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "exec-123", resp.ExecutionID)
		assert.Equal(t, "Execution kill started successfully", resp.Message)
	})
}

func TestClient_ListExecutions(t *testing.T) {
	t.Run("successful list executions with limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/executions", r.URL.Path)
			assert.Equal(t, "10", r.URL.Query().Get("limit"))

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]api.Execution{
				{ExecutionID: "exec-1", Status: "SUCCEEDED"},
				{ExecutionID: "exec-2", Status: "RUNNING"},
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		executions, err := c.ListExecutions(context.Background(), 10, "")

		require.NoError(t, err)
		require.NotNil(t, executions)
		assert.Len(t, executions, 2)
		assert.Equal(t, "exec-1", executions[0].ExecutionID)
		assert.Equal(t, "exec-2", executions[1].ExecutionID)
	})

	t.Run("list executions with status filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/executions", r.URL.Path)
			assert.Equal(t, "20", r.URL.Query().Get("limit"))
			assert.Equal(t, "RUNNING,TERMINATING", r.URL.Query().Get("status"))

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]api.Execution{
				{ExecutionID: "exec-1", Status: "RUNNING"},
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		executions, err := c.ListExecutions(context.Background(), 20, "RUNNING,TERMINATING")

		require.NoError(t, err)
		require.NotNil(t, executions)
		assert.Len(t, executions, 1)
		assert.Equal(t, "exec-1", executions[0].ExecutionID)
	})

	t.Run("list executions with unlimited limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/executions", r.URL.Path)
			assert.Equal(t, "0", r.URL.Query().Get("limit"))

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]api.Execution{
				{ExecutionID: "exec-1", Status: "SUCCEEDED"},
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		executions, err := c.ListExecutions(context.Background(), 0, "")

		require.NoError(t, err)
		require.NotNil(t, executions)
		assert.Len(t, executions, 1)
		assert.Equal(t, "exec-1", executions[0].ExecutionID)
	})
}

func TestClient_ClaimAPIKey(t *testing.T) {
	t.Run("successful API key claim", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.True(t, strings.HasPrefix(r.URL.Path, "/api/v1/claim/"))

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.ClaimAPIKeyResponse{
				APIKey:    "claimed-api-key",
				UserEmail: "user@example.com",
				Message:   "API key claimed successfully",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.ClaimAPIKey(context.Background(), "claim-token-123")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "claimed-api-key", resp.APIKey)
		assert.Equal(t, "user@example.com", resp.UserEmail)
	})
}

func TestClient_RegisterImage(t *testing.T) {
	t.Run("successful image registration", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/images/register", r.URL.Path)

			var req api.RegisterImageRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "ubuntu:22.04", req.Image)
			assert.NotNil(t, req.IsDefault)
			assert.True(t, *req.IsDefault)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.RegisterImageResponse{
				Image:   "ubuntu:22.04",
				Message: "Image registered successfully",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		isDefault := true
		resp, err := c.RegisterImage(context.Background(), "ubuntu:22.04", &isDefault, nil, nil, nil, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "ubuntu:22.04", resp.Image)
		assert.Equal(t, "Image registered successfully", resp.Message)
	})

	t.Run("register image without default flag", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req api.RegisterImageRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "ubuntu:22.04", req.Image)
			assert.Nil(t, req.IsDefault)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.RegisterImageResponse{
				Image:   "ubuntu:22.04",
				Message: "Image registered successfully",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.RegisterImage(context.Background(), "ubuntu:22.04", nil, nil, nil, nil, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("register image with task roles", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req api.RegisterImageRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "alpine:latest", req.Image)
			assert.Nil(t, req.IsDefault)
			assert.NotNil(t, req.TaskRoleName)
			assert.Equal(t, "my-task-role", *req.TaskRoleName)
			assert.NotNil(t, req.TaskExecutionRoleName)
			assert.Equal(t, "my-exec-role", *req.TaskExecutionRoleName)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.RegisterImageResponse{
				Image:   "alpine:latest",
				Message: "Image registered successfully",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		taskRole := "my-task-role"
		taskExecRole := "my-exec-role"
		resp, err := c.RegisterImage(context.Background(), "alpine:latest", nil, &taskRole, &taskExecRole, nil, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "alpine:latest", resp.Image)
		assert.Equal(t, "Image registered successfully", resp.Message)
	})
}

func TestClient_ListImages(t *testing.T) {
	t.Run("successful image list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/images", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.ListImagesResponse{
				Images: []api.ImageInfo{
					{Image: "ubuntu:22.04", IsDefault: boolPtr(true)},
					{Image: "alpine:latest", IsDefault: boolPtr(false)},
				},
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.ListImages(context.Background())

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Images, 2)
		assert.Equal(t, "ubuntu:22.04", resp.Images[0].Image)
		assert.True(t, *resp.Images[0].IsDefault)
	})
}

func TestClient_UnregisterImage(t *testing.T) {
	t.Run("successful image unregistration", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "DELETE", r.Method)
			assert.True(t, strings.HasPrefix(r.URL.Path, "/api/v1/images/"))
			assert.True(t, strings.Contains(r.URL.Path, "ubuntu:22.04"))

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.RemoveImageResponse{
				Image:   "ubuntu:22.04",
				Message: "Image unregistered successfully",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.UnregisterImage(context.Background(), "ubuntu:22.04")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "ubuntu:22.04", resp.Image)
		assert.Equal(t, "Image unregistered successfully", resp.Message)
	})
}

func TestClient_CreateSecret(t *testing.T) {
	t.Run("successful secret creation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/secrets", r.URL.Path)

			var req api.CreateSecretRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "github-token", req.Name)
			assert.Equal(t, "GITHUB_TOKEN", req.KeyName)
			assert.Equal(t, "ghp_xxxxx", req.Value)
			assert.Equal(t, "GitHub API token", req.Description)

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(api.CreateSecretResponse{
				Message: "Secret created successfully",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.CreateSecret(context.Background(), api.CreateSecretRequest{
			Name:        "github-token",
			KeyName:     "GITHUB_TOKEN",
			Value:       "ghp_xxxxx",
			Description: "GitHub API token",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "Secret created successfully", resp.Message)
	})

	t.Run("error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(api.ErrorResponse{
				Error:   "Conflict",
				Details: "Secret already exists",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.CreateSecret(context.Background(), api.CreateSecretRequest{
			Name:    "github-token",
			KeyName: "GITHUB_TOKEN",
			Value:   "ghp_xxxxx",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Conflict")
	})
}

func TestClient_GetSecret(t *testing.T) {
	t.Run("successful secret retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/secrets/github-token", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.GetSecretResponse{
				Secret: &api.Secret{
					Name:        "github-token",
					KeyName:     "GITHUB_TOKEN",
					Description: "GitHub API token",
					Value:       "ghp_xxxxx",
					CreatedBy:   "alice@example.com",
					OwnedBy:     []string{"alice@example.com"},
					CreatedAt:   time.Now().UTC(),
					UpdatedBy:   "alice@example.com",
					UpdatedAt:   time.Now().UTC(),
				},
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.GetSecret(context.Background(), "github-token")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Secret)
		assert.Equal(t, "github-token", resp.Secret.Name)
		assert.Equal(t, "GITHUB_TOKEN", resp.Secret.KeyName)
		assert.Equal(t, "ghp_xxxxx", resp.Secret.Value)
	})

	t.Run("secret not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(api.ErrorResponse{
				Error:   "Not Found",
				Details: "Secret not found",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.GetSecret(context.Background(), "nonexistent")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Not Found")
	})
}

func TestClient_ListSecrets(t *testing.T) {
	t.Run("successful list secrets", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/secrets", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.ListSecretsResponse{
				Secrets: []*api.Secret{
					{
						Name:      "github-token",
						KeyName:   "GITHUB_TOKEN",
						CreatedBy: "alice@example.com",
						CreatedAt: time.Now().UTC(),
					},
					{
						Name:      "db-password",
						KeyName:   "DB_PASSWORD",
						CreatedBy: "bob@example.com",
						CreatedAt: time.Now().UTC(),
					},
				},
				Total: 2,
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.ListSecrets(context.Background())

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Secrets, 2)
		assert.Equal(t, 2, resp.Total)
		assert.Equal(t, "github-token", resp.Secrets[0].Name)
		assert.Equal(t, "db-password", resp.Secrets[1].Name)
	})

	t.Run("empty list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.ListSecretsResponse{
				Secrets: []*api.Secret{},
				Total:   0,
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.ListSecrets(context.Background())

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Secrets, 0)
		assert.Equal(t, 0, resp.Total)
	})
}

func TestClient_UpdateSecret(t *testing.T) {
	t.Run("successful secret update", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			assert.Equal(t, "/api/v1/secrets/github-token", r.URL.Path)

			var req api.UpdateSecretRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "GITHUB_API_TOKEN", req.KeyName)
			assert.Equal(t, "new-token-value", req.Value)
			assert.Equal(t, "Updated description", req.Description)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.UpdateSecretResponse{
				Message: "Secret updated successfully",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.UpdateSecret(context.Background(), "github-token", api.UpdateSecretRequest{
			KeyName:     "GITHUB_API_TOKEN",
			Value:       "new-token-value",
			Description: "Updated description",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "Secret updated successfully", resp.Message)
	})

	t.Run("update only value", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req api.UpdateSecretRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "new-value", req.Value)
			assert.Empty(t, req.KeyName)
			assert.Empty(t, req.Description)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.UpdateSecretResponse{
				Message: "Secret updated successfully",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.UpdateSecret(context.Background(), "github-token", api.UpdateSecretRequest{
			Value: "new-value",
		})

		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestClient_DeleteSecret(t *testing.T) {
	t.Run("successful secret deletion", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "DELETE", r.Method)
			assert.Equal(t, "/api/v1/secrets/github-token", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.DeleteSecretResponse{
				Name:    "github-token",
				Message: "Secret deleted successfully",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.DeleteSecret(context.Background(), "github-token")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "github-token", resp.Name)
		assert.Equal(t, "Secret deleted successfully", resp.Message)
	})

	t.Run("secret not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(api.ErrorResponse{
				Error:   "Not Found",
				Details: "Secret not found",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.DeleteSecret(context.Background(), "nonexistent")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Not Found")
	})
}

func TestClient_ReconcileHealth(t *testing.T) {
	t.Run("successful health reconciliation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/health/reconcile", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.HealthReconcileResponse{
				Status: "completed",
				Report: &api.HealthReport{
					Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
					ComputeStatus: api.ComputeHealthStatus{
						TotalResources:  5,
						VerifiedCount:   4,
						RecreatedCount:  1,
						TagUpdatedCount: 0,
					},
					ReconciledCount: 5,
					ErrorCount:      0,
				},
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.ReconcileHealth(context.Background())

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "completed", resp.Status)
		require.NotNil(t, resp.Report)
		assert.Equal(t, 5, resp.Report.ComputeStatus.TotalResources)
		assert.Equal(t, 4, resp.Report.ComputeStatus.VerifiedCount)
		assert.Equal(t, 1, resp.Report.ComputeStatus.RecreatedCount)
	})

	t.Run("error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(api.ErrorResponse{
				Error:   "Unauthorized",
				Details: "Invalid API key",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "invalid-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.ReconcileHealth(context.Background())

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Unauthorized")
	})
}

func TestClient_GetImage(t *testing.T) {
	t.Run("successful image retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.True(t, strings.HasPrefix(r.URL.Path, "/api/v1/images/"))

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.ImageInfo{
				Image:     "ubuntu:22.04",
				IsDefault: boolPtr(true),
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.GetImage(context.Background(), "ubuntu:22.04")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "ubuntu:22.04", resp.Image)
		assert.NotNil(t, resp.IsDefault)
		assert.True(t, *resp.IsDefault)
	})

	t.Run("image not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(api.ErrorResponse{
				Error:   "Not Found",
				Details: "Image not found",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.GetImage(context.Background(), "nonexistent:latest")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Not Found")
	})

	t.Run("image name with special characters is URL encoded", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			// r.URL.Path is decoded by Go's HTTP server, so check for decoded path
			// This verifies that the client properly encoded the path and the server decoded it
			assert.True(t, strings.Contains(r.URL.Path, "repo/image"))
			assert.True(t, strings.Contains(r.URL.Path, "repo/image:tag"))

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.ImageInfo{
				Image:     "repo/image:tag",
				IsDefault: boolPtr(false),
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.GetImage(context.Background(), "repo/image:tag")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "repo/image:tag", resp.Image)
	})
}

func TestClient_KillExecution_NoContent(t *testing.T) {
	t.Run("execution already terminated returns nil", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "DELETE", r.Method)
			assert.Equal(t, "/api/v1/executions/exec-already-done", r.URL.Path)

			// Return 204 No Content for already terminated execution
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.KillExecution(context.Background(), "exec-already-done")

		require.NoError(t, err)
		assert.Nil(t, resp)
	})

	t.Run("kill execution error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(api.ErrorResponse{
				Error:   "Not Found",
				Details: "Execution not found",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.KillExecution(context.Background(), "nonexistent")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Not Found")
	})

	t.Run("kill execution invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`invalid json`))
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.KillExecution(context.Background(), "exec-123")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to parse response")
	})
}

func TestClient_DoJSON_NoContent(t *testing.T) {
	t.Run("204 No Content response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		var result api.ExecutionStatusResponse
		err := c.DoJSON(context.Background(), Request{
			Method: "POST",
			Path:   "/api/v1/test",
		}, &result)

		require.NoError(t, err)
		// Result should be unmodified (zero value)
		assert.Empty(t, result.ExecutionID)
	})
}

func TestClient_buildURL(t *testing.T) {
	tests := []struct {
		name        string
		apiEndpoint string
		path        string
		want        string
		wantErr     bool
	}{
		{
			name:        "simple path",
			apiEndpoint: "https://api.example.com",
			path:        "/api/v1/health",
			want:        "https://api.example.com/api/v1/health",
			wantErr:     false,
		},
		{
			name:        "path with query string",
			apiEndpoint: "https://api.example.com",
			path:        "/api/v1/executions?limit=10&status=RUNNING",
			want:        "https://api.example.com/api/v1/executions?limit=10&status=RUNNING",
			wantErr:     false,
		},
		{
			name:        "path with multiple query parameters",
			apiEndpoint: "https://api.example.com",
			path:        "/api/v1/users?role=admin&active=true&limit=50",
			want:        "https://api.example.com/api/v1/users?role=admin&active=true&limit=50",
			wantErr:     false,
		},
		{
			name:        "path without leading slash",
			apiEndpoint: "https://api.example.com",
			path:        "api/v1/health",
			want:        "https://api.example.com/api/v1/health",
			wantErr:     false,
		},
		{
			name:        "endpoint with trailing slash",
			apiEndpoint: "https://api.example.com/",
			path:        "/api/v1/health",
			want:        "https://api.example.com/api/v1/health",
			wantErr:     false,
		},
		{
			name:        "empty path with query string",
			apiEndpoint: "https://api.example.com",
			path:        "?query=test",
			want:        "https://api.example.com?query=test",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				APIEndpoint: tt.apiEndpoint,
				APIKey:      "test-api-key",
			}
			c := New(cfg, testutil.SilentLogger())

			got, err := c.buildURL(tt.path)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestClient_Do_WithContext(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := c.Do(ctx, Request{
			Method: "GET",
			Path:   "/api/v1/test",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestClient_Do_QueryString(t *testing.T) {
	t.Run("request with query parameters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "10", r.URL.Query().Get("limit"))
			assert.Equal(t, "RUNNING", r.URL.Query().Get("status"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result": "ok"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			APIEndpoint: server.URL,
			APIKey:      "test-api-key",
		}
		c := New(cfg, testutil.SilentLogger())

		resp, err := c.Do(context.Background(), Request{
			Method: "GET",
			Path:   "/api/v1/executions?limit=10&status=RUNNING",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
