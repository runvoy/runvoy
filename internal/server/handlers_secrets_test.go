package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/backend/orchestrator"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock secret repository for testing
type testSecretRepository struct {
	createSecretFunc func(ctx context.Context, secret *api.Secret) error
	getSecretFunc    func(ctx context.Context, name string, includeValue bool) (*api.Secret, error)
	listSecretsFunc  func(ctx context.Context, includeValue bool) ([]*api.Secret, error)
	updateSecretFunc func(ctx context.Context, secret *api.Secret) error
	deleteSecretFunc func(ctx context.Context, name string) error
}

func (t *testSecretRepository) CreateSecret(ctx context.Context, secret *api.Secret) error {
	if t.createSecretFunc != nil {
		return t.createSecretFunc(ctx, secret)
	}
	return nil
}

func (t *testSecretRepository) GetSecret(ctx context.Context, name string, includeValue bool) (*api.Secret, error) {
	if t.getSecretFunc != nil {
		return t.getSecretFunc(ctx, name, includeValue)
	}
	return &api.Secret{
		Name: name,
	}, nil
}

func (t *testSecretRepository) ListSecrets(ctx context.Context, includeValue bool) ([]*api.Secret, error) {
	if t.listSecretsFunc != nil {
		return t.listSecretsFunc(ctx, includeValue)
	}
	return []*api.Secret{}, nil
}

func (t *testSecretRepository) UpdateSecret(ctx context.Context, secret *api.Secret) error {
	if t.updateSecretFunc != nil {
		return t.updateSecretFunc(ctx, secret)
	}
	return nil
}

func (t *testSecretRepository) DeleteSecret(ctx context.Context, name string) error {
	if t.deleteSecretFunc != nil {
		return t.deleteSecretFunc(ctx, name)
	}
	return nil
}

func TestHandleCreateSecret_Success(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	createReq := api.CreateSecretRequest{
		Name:        "test-secret",
		Value:       "secret-value",
		Description: "Test secret",
		KeyName:     "TEST_SECRET",
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/api/v1/secrets", bytes.NewReader(body))
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp api.CreateSecretResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "Secret created successfully", resp.Message)
}

// Note: Authentication tests (MissingUser) are not included here as they require
// proper API key authentication through the middleware, which is tested separately
// in the auth package tests.

func TestHandleCreateSecret_InvalidRequestBody(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	req := httptest.NewRequest("POST", "/api/v1/secrets", bytes.NewReader([]byte("invalid json")))
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateSecret_ServiceError(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{
		createSecretFunc: func(_ context.Context, _ *api.Secret) error {
			return apperrors.ErrInternalError("database error", errors.New("DB connection failed"))
		},
	}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	createReq := api.CreateSecretRequest{
		Name:  "test-secret",
		Value: "secret-value",
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/api/v1/secrets", bytes.NewReader(body))
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleGetSecret_Success(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{
		getSecretFunc: func(_ context.Context, name string, _ bool) (*api.Secret, error) {
			return &api.Secret{
				Name:        name,
				Value:       "secret-value",
				Description: "Test secret",
				KeyName:     "TEST_KEY",
				OwnedBy:     []string{"user@example.com"},
			}, nil
		},
	}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	req := httptest.NewRequest("GET", "/api/v1/secrets/my-secret", http.NoBody)
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.GetSecretResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "my-secret", resp.Secret.Name)
	assert.Equal(t, "secret-value", resp.Secret.Value)
	assert.Equal(t, []string{"user@example.com"}, resp.Secret.OwnedBy)
}

func TestHandleGetSecret_NotFound(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{
		getSecretFunc: func(_ context.Context, _ string, _ bool) (*api.Secret, error) {
			return nil, apperrors.ErrNotFound("secret not found", nil)
		},
	}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	req := httptest.NewRequest("GET", "/api/v1/secrets/nonexistent", http.NoBody)
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleListSecrets_Success(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{
		listSecretsFunc: func(_ context.Context, _ bool) ([]*api.Secret, error) {
			return []*api.Secret{
				{Name: "secret1", Description: "First secret", CreatedBy: "user@example.com"},
				{Name: "secret2", Description: "Second secret", CreatedBy: "user@example.com"},
			}, nil
		},
	}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	req := httptest.NewRequest("GET", "/api/v1/secrets", http.NoBody)
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.ListSecretsResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Secrets, 2)
}

func TestHandleListSecrets_Empty(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{
		listSecretsFunc: func(_ context.Context, _ bool) ([]*api.Secret, error) {
			return []*api.Secret{}, nil
		},
	}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	req := httptest.NewRequest("GET", "/api/v1/secrets", http.NoBody)
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.ListSecretsResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, 0, resp.Total)
}

func TestHandleListSecrets_ServiceError(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{
		// Return empty list during initialization (no error)
		listSecretsFunc: func(_ context.Context, includeValue bool) ([]*api.Secret, error) {
			// During initialization, includeValue is false, so return empty list
			if !includeValue {
				return []*api.Secret{}, nil
			}
			// During handler call, includeValue is true, so return error
			return nil, apperrors.ErrDatabaseError("database connection failed", errors.New("connection timeout"))
		},
	}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	req := httptest.NewRequest("GET", "/api/v1/secrets", http.NoBody)
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleUpdateSecret_Success(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{
		updateSecretFunc: func(_ context.Context, _ *api.Secret) error {
			return nil
		},
	}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	updateReq := api.UpdateSecretRequest{
		Value:       "updated-value",
		Description: "Updated description",
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest("PUT", "/api/v1/secrets/my-secret", bytes.NewReader(body))
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.UpdateSecretResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "Secret updated successfully", resp.Message)
}

func TestHandleUpdateSecret_InvalidBody(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	req := httptest.NewRequest("PUT", "/api/v1/secrets/my-secret", bytes.NewReader([]byte("invalid json")))
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateSecret_NotFound(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{
		updateSecretFunc: func(_ context.Context, _ *api.Secret) error {
			return apperrors.ErrNotFound("secret not found", nil)
		},
	}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	updateReq := api.UpdateSecretRequest{
		Value: "updated-value",
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest("PUT", "/api/v1/secrets/nonexistent", bytes.NewReader(body))
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleDeleteSecret_Success(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{
		deleteSecretFunc: func(_ context.Context, _ string) error {
			return nil
		},
	}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	req := httptest.NewRequest("DELETE", "/api/v1/secrets/my-secret", http.NoBody)
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.DeleteSecretResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "my-secret", resp.Name)
	assert.Equal(t, "Secret deleted successfully", resp.Message)
}

func TestHandleDeleteSecret_NotFound(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	secretRepo := &testSecretRepository{
		deleteSecretFunc: func(_ context.Context, _ string) error {
			return apperrors.ErrNotFound("secret not found", nil)
		},
	}

	svc := newTestService(t, userRepo, execRepo, secretRepo)
	router := NewRouter(svc, 30*1000)

	req := httptest.NewRequest("DELETE", "/api/v1/secrets/nonexistent", http.NoBody)
	req = addAuthToRequest(req)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &api.User{Email: "user@example.com"}))

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Helper function to add authentication to request
func addAuthToRequest(req *http.Request) *http.Request {
	req.Header.Set("X-API-Key", "test-api-key")
	return req
}

// Create a test service with the given repositories
func newTestService(
	t *testing.T,
	userRepo *testUserRepository,
	execRepo *testExecutionRepository,
	secretRepo *testSecretRepository,
) *orchestrator.Service {
	logger := testutil.SilentLogger()

	// Create a mock runner that implements the Runner interface
	mockRunner := &testRunner{}
	tokenRepo := &testTokenRepository{}
	enforcer := newPermissiveTestEnforcer(t)

	// Use a user repository with roles for enforcer initialization
	userRepoWithRoles := &testUserRepositoryWithRolesForSecrets{
		originalRepo: userRepo,
	}

	svc, err := orchestrator.NewService(context.Background(),
		userRepoWithRoles,
		execRepo,
		nil, // connRepo
		tokenRepo,
		mockRunner,
		logger,
		constants.AWS,
		nil, // wsManager
		secretRepo,
		nil, // healthManager
		enforcer,
	)
	require.NoError(t, err)
	return svc
}

// testUserRepositoryWithRolesForSecrets wraps a testUserRepository and adds ListUsers with roles
// for enforcer initialization while preserving other test behavior
type testUserRepositoryWithRolesForSecrets struct {
	originalRepo *testUserRepository
}

func (t *testUserRepositoryWithRolesForSecrets) CreateUser(
	ctx context.Context, user *api.User, key string, exp int64) error {
	return t.originalRepo.CreateUser(ctx, user, key, exp)
}

func (t *testUserRepositoryWithRolesForSecrets) RemoveExpiration(ctx context.Context, email string) error {
	return t.originalRepo.RemoveExpiration(ctx, email)
}

func (t *testUserRepositoryWithRolesForSecrets) GetUserByEmail(ctx context.Context, email string) (*api.User, error) {
	return t.originalRepo.GetUserByEmail(ctx, email)
}

func (t *testUserRepositoryWithRolesForSecrets) GetUserByAPIKeyHash(
	ctx context.Context, hash string) (*api.User, error) {
	return t.originalRepo.GetUserByAPIKeyHash(ctx, hash)
}

func (t *testUserRepositoryWithRolesForSecrets) UpdateLastUsed(ctx context.Context, email string) (*time.Time, error) {
	return t.originalRepo.UpdateLastUsed(ctx, email)
}

func (t *testUserRepositoryWithRolesForSecrets) RevokeUser(ctx context.Context, email string) error {
	return t.originalRepo.RevokeUser(ctx, email)
}

func (t *testUserRepositoryWithRolesForSecrets) CreatePendingAPIKey(ctx context.Context, key *api.PendingAPIKey) error {
	return t.originalRepo.CreatePendingAPIKey(ctx, key)
}

func (t *testUserRepositoryWithRolesForSecrets) GetPendingAPIKey(
	ctx context.Context, token string) (*api.PendingAPIKey, error) {
	return t.originalRepo.GetPendingAPIKey(ctx, token)
}

func (t *testUserRepositoryWithRolesForSecrets) MarkAsViewed(ctx context.Context, token, ip string) error {
	return t.originalRepo.MarkAsViewed(ctx, token, ip)
}

func (t *testUserRepositoryWithRolesForSecrets) DeletePendingAPIKey(ctx context.Context, token string) error {
	return t.originalRepo.DeletePendingAPIKey(ctx, token)
}

func (t *testUserRepositoryWithRolesForSecrets) ListUsers(_ context.Context) ([]*api.User, error) {
	// Return users with valid roles for enforcer initialization
	return []*api.User{
		{
			Email:     "user@example.com",
			Role:      "admin",
			CreatedAt: time.Now().Add(-24 * time.Hour),
			Revoked:   false,
		},
	}, nil
}
