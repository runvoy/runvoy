package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/constants"
	apperrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUserHandlerRouter(t *testing.T, userRepo *testUserRepository) *Router {
	if userRepo == nil {
		userRepo = &testUserRepository{}
	}
	svc := newTestOrchestratorService(t, userRepo, &testExecutionRepository{}, nil, &testRunner{}, nil, nil, nil)
	return &Router{svc: svc}
}

func addAuthenticatedUser(req *http.Request, user *api.User) *http.Request {
	ctx := context.WithValue(req.Context(), userContextKey, user)
	return req.WithContext(ctx)
}

func adminTestUser() *api.User {
	return testutil.NewUserBuilder().
		WithEmail("admin@example.com").
		WithRole("admin").
		Build()
}

func TestHandleCreateUser_Success(t *testing.T) {
	userRepo := &testUserRepository{
		getUserByEmailFunc: func(_ string) (*api.User, error) {
			return nil, nil
		},
	}
	router := newUserHandlerRouter(t, userRepo)

	reqBody := api.CreateUserRequest{
		Email: "newuser@example.com",
		Role:  "developer",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, adminTestUser())

	w := httptest.NewRecorder()
	router.handleCreateUser(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response api.CreateUserResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	require.NotNil(t, response.User)
	assert.Equal(t, reqBody.Email, response.User.Email)
	assert.Equal(t, reqBody.Role, response.User.Role)
	assert.NotEmpty(t, response.ClaimToken)
}

func TestHandleCreateUser_InvalidJSON(t *testing.T) {
	router := newUserHandlerRouter(t, &testUserRepository{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, adminTestUser())

	w := httptest.NewRecorder()
	router.handleCreateUser(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateUser_NoAuthentication(t *testing.T) {
	router := newUserHandlerRouter(t, &testUserRepository{
		getUserByEmailFunc: func(_ string) (*api.User, error) {
			return nil, nil
		},
	})

	reqBody := api.CreateUserRequest{
		Email: "newuser@example.com",
		Role:  "developer",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.handleCreateUser(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandleCreateUser_ServiceError(t *testing.T) {
	userRepo := &testUserRepository{
		getUserByEmailFunc: func(_ string) (*api.User, error) {
			return nil, nil
		},
		createUserFunc: func(_ context.Context, _ *api.User, _ string, _ int64) error {
			return apperrors.ErrDatabaseError("database error", nil)
		},
	}
	router := newUserHandlerRouter(t, userRepo)

	reqBody := api.CreateUserRequest{
		Email: "newuser@example.com",
		Role:  "developer",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, adminTestUser())

	w := httptest.NewRecorder()
	router.handleCreateUser(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleRevokeUser_Success(t *testing.T) {
	router := newUserHandlerRouter(t, &testUserRepository{})

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

	var response api.RevokeUserResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, reqBody.Email, response.Email)
	assert.Contains(t, response.Message, "revoked")
}

func TestHandleRevokeUser_InvalidJSON(t *testing.T) {
	router := newUserHandlerRouter(t, &testUserRepository{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.handleRevokeUser(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleRevokeUser_ServiceError(t *testing.T) {
	userRepo := &testUserRepository{
		revokeUserFunc: func(_ context.Context, _ string) error {
			return apperrors.ErrInternalError("failed to revoke user", nil)
		},
	}
	router := newUserHandlerRouter(t, userRepo)

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
	router := newUserHandlerRouter(t, &testUserRepository{})

	reqBody := api.RevokeUserRequest{
		Email: "",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.handleRevokeUser(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleListUsers_Success(t *testing.T) {
	expectedUsers := []*api.User{
		{
			Email: "user1@example.com",
			Role:  "admin",
		},
		{
			Email: "user2@example.com",
			Role:  "developer",
		},
	}
	userRepo := &testUserRepository{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return expectedUsers, nil
		},
	}
	router := newUserHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	req = addAuthenticatedUser(req, adminTestUser())

	w := httptest.NewRecorder()
	router.handleListUsers(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.ListUsersResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response.Users, 2)
	assert.Equal(t, expectedUsers[0].Email, response.Users[0].Email)
	assert.Equal(t, expectedUsers[1].Email, response.Users[1].Email)
}

func TestHandleListUsers_EmptyList(t *testing.T) {
	userRepo := &testUserRepository{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return []*api.User{}, nil
		},
	}
	router := newUserHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	req = addAuthenticatedUser(req, adminTestUser())

	w := httptest.NewRecorder()
	router.handleListUsers(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.ListUsersResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response.Users, 0)
}

func TestHandleListUsers_NoAuthentication(t *testing.T) {
	userRepo := &testUserRepository{}
	svc := newTestOrchestratorService(t, userRepo, &testExecutionRepository{}, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/", http.NoBody)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestHandleListUsers_ServiceError(t *testing.T) {
	var hydrated bool
	userRepo := &testUserRepository{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			if !hydrated {
				hydrated = true
				return []*api.User{
					{Email: "seed@example.com", Role: "admin"},
				}, nil
			}
			return nil, apperrors.ErrDatabaseError("database connection failed", nil)
		},
	}
	router := newUserHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
	req = addAuthenticatedUser(req, adminTestUser())

	w := httptest.NewRecorder()
	router.handleListUsers(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleCreateUser_DifferentRoles(t *testing.T) {
	roles := []string{"admin", "developer", "viewer"}

	for _, role := range roles {
		t.Run("role_"+role, func(t *testing.T) {
			userRepo := &testUserRepository{
				getUserByEmailFunc: func(_ string) (*api.User, error) {
					return nil, nil
				},
			}
			router := newUserHandlerRouter(t, userRepo)

			reqBody := api.CreateUserRequest{
				Email: "user@example.com",
				Role:  role,
			}
			body, err := json.Marshal(reqBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = addAuthenticatedUser(req, adminTestUser())

			w := httptest.NewRecorder()
			router.handleCreateUser(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)

			var response api.CreateUserResponse
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			require.NotNil(t, response.User)
			assert.Equal(t, role, response.User.Role)
		})
	}
}
