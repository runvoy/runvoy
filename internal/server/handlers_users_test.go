package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServiceForUsers is a mock service for testing user handlers
type mockServiceForUsers struct {
	createUserFunc func(ctx context.Context, req api.CreateUserRequest, creatorEmail string) (*api.CreateUserResponse, error)
	revokeUserFunc func(ctx context.Context, email string) error
	listUsersFunc  func(ctx context.Context) (*api.ListUsersResponse, error)
}

func (m *mockServiceForUsers) CreateUser(ctx context.Context, req api.CreateUserRequest, creatorEmail string) (*api.CreateUserResponse, error) {
	if m.createUserFunc != nil {
		return m.createUserFunc(ctx, req, creatorEmail)
	}
	return nil, nil
}

func (m *mockServiceForUsers) RevokeUser(ctx context.Context, email string) error {
	if m.revokeUserFunc != nil {
		return m.revokeUserFunc(ctx, email)
	}
	return nil
}

func (m *mockServiceForUsers) ListUsers(ctx context.Context) (*api.ListUsersResponse, error) {
	if m.listUsersFunc != nil {
		return m.listUsersFunc(ctx)
	}
	return &api.ListUsersResponse{Users: []api.User{}}, nil
}

func TestHandleCreateUser_Success(t *testing.T) {
	expectedResponse := &api.CreateUserResponse{
		User: api.User{
			Email:     "newuser@example.com",
			Role:      "developer",
			CreatedAt: "2024-01-01T00:00:00Z",
		},
		APIKey: "test-api-key-123",
	}

	router := &Router{
		svc: &mockServiceForUsers{
			createUserFunc: func(ctx context.Context, req api.CreateUserRequest, creatorEmail string) (*api.CreateUserResponse, error) {
				assert.Equal(t, "newuser@example.com", req.Email)
				assert.Equal(t, "developer", req.Role)
				return expectedResponse, nil
			},
		},
	}

	// Create request body
	reqBody := api.CreateUserRequest{
		Email: "newuser@example.com",
		Role:  "developer",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Add authenticated user to context
	user := testutil.NewUserBuilder().
		WithEmail("admin@example.com").
		WithRole("admin").
		Build()
	ctx := context.WithValue(req.Context(), userContextKey, &user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	router.handleCreateUser(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response api.CreateUserResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, expectedResponse.User.Email, response.User.Email)
	assert.Equal(t, expectedResponse.User.Role, response.User.Role)
	assert.Equal(t, expectedResponse.APIKey, response.APIKey)
}

func TestHandleCreateUser_InvalidJSON(t *testing.T) {
	router := &Router{
		svc: &mockServiceForUsers{},
	}

	// Invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	// Add authenticated user
	user := testutil.NewUserBuilder().WithEmail("admin@example.com").Build()
	ctx := context.WithValue(req.Context(), userContextKey, &user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	router.handleCreateUser(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateUser_NoAuthentication(t *testing.T) {
	router := &Router{
		svc: &mockServiceForUsers{},
	}

	reqBody := api.CreateUserRequest{
		Email: "newuser@example.com",
		Role:  "developer",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// No authenticated user in context
	w := httptest.NewRecorder()

	router.handleCreateUser(w, req)

	// Should return unauthorized or forbidden
	assert.True(t, w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden)
}

func TestHandleCreateUser_ServiceError(t *testing.T) {
	router := &Router{
		svc: &mockServiceForUsers{
			createUserFunc: func(ctx context.Context, req api.CreateUserRequest, creatorEmail string) (*api.CreateUserResponse, error) {
				return nil, errors.New("database error")
			},
		},
	}

	reqBody := api.CreateUserRequest{
		Email: "newuser@example.com",
		Role:  "developer",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	user := testutil.NewUserBuilder().WithEmail("admin@example.com").Build()
	ctx := context.WithValue(req.Context(), userContextKey, &user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	router.handleCreateUser(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleRevokeUser_Success(t *testing.T) {
	revoked := false
	router := &Router{
		svc: &mockServiceForUsers{
			revokeUserFunc: func(ctx context.Context, email string) error {
				assert.Equal(t, "user@example.com", email)
				revoked = true
				return nil
			},
		},
	}

	reqBody := api.RevokeUserRequest{
		Email: "user@example.com",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	router.handleRevokeUser(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, revoked)

	var response api.RevokeUserResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Contains(t, response.Message, "revoked")
	assert.Equal(t, "user@example.com", response.Email)
}

func TestHandleRevokeUser_InvalidJSON(t *testing.T) {
	router := &Router{
		svc: &mockServiceForUsers{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	router.handleRevokeUser(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleRevokeUser_ServiceError(t *testing.T) {
	router := &Router{
		svc: &mockServiceForUsers{
			revokeUserFunc: func(ctx context.Context, email string) error {
				return errors.New("user not found")
			},
		},
	}

	reqBody := api.RevokeUserRequest{
		Email: "nonexistent@example.com",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	router.handleRevokeUser(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleRevokeUser_EmptyEmail(t *testing.T) {
	router := &Router{
		svc: &mockServiceForUsers{
			revokeUserFunc: func(ctx context.Context, email string) error {
				if email == "" {
					return errors.New("email is required")
				}
				return nil
			},
		},
	}

	reqBody := api.RevokeUserRequest{
		Email: "",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	router.handleRevokeUser(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleListUsers_Success(t *testing.T) {
	expectedUsers := []api.User{
		{
			Email:     "user1@example.com",
			Role:      "admin",
			CreatedAt: "2024-01-01T00:00:00Z",
		},
		{
			Email:     "user2@example.com",
			Role:      "developer",
			CreatedAt: "2024-01-02T00:00:00Z",
		},
	}

	router := &Router{
		svc: &mockServiceForUsers{
			listUsersFunc: func(ctx context.Context) (*api.ListUsersResponse, error) {
				return &api.ListUsersResponse{
					Users: expectedUsers,
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)

	// Add authenticated user
	user := testutil.NewUserBuilder().WithEmail("admin@example.com").Build()
	ctx := context.WithValue(req.Context(), userContextKey, &user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	router.handleListUsers(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.ListUsersResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Len(t, response.Users, 2)
	assert.Equal(t, "user1@example.com", response.Users[0].Email)
	assert.Equal(t, "user2@example.com", response.Users[1].Email)
}

func TestHandleListUsers_EmptyList(t *testing.T) {
	router := &Router{
		svc: &mockServiceForUsers{
			listUsersFunc: func(ctx context.Context) (*api.ListUsersResponse, error) {
				return &api.ListUsersResponse{
					Users: []api.User{},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)

	user := testutil.NewUserBuilder().WithEmail("admin@example.com").Build()
	ctx := context.WithValue(req.Context(), userContextKey, &user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	router.handleListUsers(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.ListUsersResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Len(t, response.Users, 0)
}

func TestHandleListUsers_NoAuthentication(t *testing.T) {
	router := &Router{
		svc: &mockServiceForUsers{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()

	router.handleListUsers(w, req)

	// Should return unauthorized
	assert.True(t, w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden)
}

func TestHandleListUsers_ServiceError(t *testing.T) {
	router := &Router{
		svc: &mockServiceForUsers{
			listUsersFunc: func(ctx context.Context) (*api.ListUsersResponse, error) {
				return nil, errors.New("database connection failed")
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)

	user := testutil.NewUserBuilder().WithEmail("admin@example.com").Build()
	ctx := context.WithValue(req.Context(), userContextKey, &user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	router.handleListUsers(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleCreateUser_DifferentRoles(t *testing.T) {
	roles := []string{"admin", "developer", "viewer"}

	for _, role := range roles {
		t.Run("role_"+role, func(t *testing.T) {
			router := &Router{
				svc: &mockServiceForUsers{
					createUserFunc: func(ctx context.Context, req api.CreateUserRequest, creatorEmail string) (*api.CreateUserResponse, error) {
						return &api.CreateUserResponse{
							User: api.User{
								Email: req.Email,
								Role:  req.Role,
							},
							APIKey: "test-key",
						}, nil
					},
				},
			}

			reqBody := api.CreateUserRequest{
				Email: "user@example.com",
				Role:  role,
			}
			body, err := json.Marshal(reqBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			user := testutil.NewUserBuilder().WithEmail("admin@example.com").Build()
			ctx := context.WithValue(req.Context(), userContextKey, &user)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			router.handleCreateUser(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)

			var response api.CreateUserResponse
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, role, response.User.Role)
		})
	}
}

// BenchmarkHandleCreateUser measures user creation performance
func BenchmarkHandleCreateUser(b *testing.B) {
	router := &Router{
		svc: &mockServiceForUsers{
			createUserFunc: func(ctx context.Context, req api.CreateUserRequest, creatorEmail string) (*api.CreateUserResponse, error) {
				return &api.CreateUserResponse{
					User: api.User{
						Email: req.Email,
						Role:  req.Role,
					},
					APIKey: "test-key",
				}, nil
			},
		},
	}

	reqBody := api.CreateUserRequest{
		Email: "bench@example.com",
		Role:  "developer",
	}
	body, _ := json.Marshal(reqBody)

	user := testutil.NewUserBuilder().WithEmail("admin@example.com").Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), userContextKey, &user)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		router.handleCreateUser(w, req)
	}
}

// BenchmarkHandleListUsers measures user listing performance
func BenchmarkHandleListUsers(b *testing.B) {
	users := make([]api.User, 100)
	for i := 0; i < 100; i++ {
		users[i] = api.User{
			Email:     "user" + string(rune(i)) + "@example.com",
			Role:      "developer",
			CreatedAt: "2024-01-01T00:00:00Z",
		}
	}

	router := &Router{
		svc: &mockServiceForUsers{
			listUsersFunc: func(ctx context.Context) (*api.ListUsersResponse, error) {
				return &api.ListUsersResponse{Users: users}, nil
			},
		},
	}

	user := testutil.NewUserBuilder().WithEmail("admin@example.com").Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		ctx := context.WithValue(req.Context(), userContextKey, &user)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		router.handleListUsers(w, req)
	}
}
