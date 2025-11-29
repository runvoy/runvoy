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

	"github.com/runvoy/runvoy/internal/api"
	apperrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper to create a router with image-related dependencies
func newImageHandlerRouter(t *testing.T, runner *testRunner) *Router {
	if runner == nil {
		runner = &testRunner{}
	}
	svc := newTestOrchestratorService(
		t,
		&testUserRepository{},
		&testExecutionRepository{},
		nil,
		runner,
		nil,
		nil,
		nil,
	)
	return &Router{svc: svc}
}

// ==================== handleRegisterImage tests ====================

func TestHandleRegisterImage_Success(t *testing.T) {
	runner := &testRunner{}
	router := newImageHandlerRouter(t, runner)

	reqBody := api.RegisterImageRequest{
		Image: "alpine:latest",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "admin",
	})

	w := httptest.NewRecorder()
	router.handleRegisterImage(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response api.RegisterImageResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "alpine:latest", response.Image)
	assert.NotEmpty(t, response.Message)
}

func TestHandleRegisterImage_WithAllOptions(t *testing.T) {
	runner := &testRunner{}
	router := newImageHandlerRouter(t, runner)

	isDefault := true
	cpu := 512
	memory := 1024
	taskRole := "arn:aws:iam::123456789:role/task-role"
	taskExecRole := "arn:aws:iam::123456789:role/exec-role"
	runtime := "LINUX/ARM64"

	reqBody := api.RegisterImageRequest{
		Image:                 "myapp:v1.0",
		IsDefault:             &isDefault,
		TaskRoleName:          &taskRole,
		TaskExecutionRoleName: &taskExecRole,
		CPU:                   &cpu,
		Memory:                &memory,
		RuntimePlatform:       &runtime,
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "admin",
	})

	w := httptest.NewRecorder()
	router.handleRegisterImage(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHandleRegisterImage_NoAuthentication(t *testing.T) {
	router := newImageHandlerRouter(t, nil)

	reqBody := api.RegisterImageRequest{
		Image: "alpine:latest",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.handleRegisterImage(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandleRegisterImage_InvalidJSON(t *testing.T) {
	router := newImageHandlerRouter(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "admin",
	})

	w := httptest.NewRecorder()
	router.handleRegisterImage(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleRegisterImage_MissingImageName(t *testing.T) {
	router := newImageHandlerRouter(t, nil)

	reqBody := api.RegisterImageRequest{
		Image: "", // Empty image name
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "admin",
	})

	w := httptest.NewRecorder()
	router.handleRegisterImage(w, req)

	// Expect validation error
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
}

func TestHandleRegisterImage_ECRImage(t *testing.T) {
	runner := &testRunner{}
	router := newImageHandlerRouter(t, runner)

	reqBody := api.RegisterImageRequest{
		Image: "123456789.dkr.ecr.us-east-1.amazonaws.com/myapp:v1.0",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "developer",
	})

	w := httptest.NewRecorder()
	router.handleRegisterImage(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

// ==================== handleListImages tests ====================

func TestHandleListImages_Success(t *testing.T) {
	now := time.Now()
	runner := &testRunner{
		listImagesFunc: func() ([]api.ImageInfo, error) {
			return []api.ImageInfo{
				{
					ImageID:   "img-1",
					Image:     "alpine:latest",
					CreatedBy: "user@example.com",
					CreatedAt: now,
				},
				{
					ImageID:   "img-2",
					Image:     "python:3.9",
					CreatedBy: "user@example.com",
					CreatedAt: now.Add(-1 * time.Hour),
				},
			}, nil
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", http.NoBody)
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "developer",
	})

	w := httptest.NewRecorder()
	router.handleListImages(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.ListImagesResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response.Images, 2)
	assert.Equal(t, "img-1", response.Images[0].ImageID)
	assert.Equal(t, "img-2", response.Images[1].ImageID)
}

func TestHandleListImages_EmptyList(t *testing.T) {
	runner := &testRunner{
		listImagesFunc: func() ([]api.ImageInfo, error) {
			return []api.ImageInfo{}, nil
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", http.NoBody)
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "developer",
	})

	w := httptest.NewRecorder()
	router.handleListImages(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.ListImagesResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response.Images, 0)
}

func TestHandleListImages_NoAuthentication(t *testing.T) {
	router := newImageHandlerRouter(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", http.NoBody)

	w := httptest.NewRecorder()
	router.handleListImages(w, req)

	// Handlers don't check auth directly - it's handled by middleware
	// When called directly without middleware, returns 200
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleListImages_ServiceError(t *testing.T) {
	callCount := 0
	runner := &testRunner{
		listImagesFunc: func() ([]api.ImageInfo, error) {
			callCount++
			// First call is during enforcer initialization, return empty list
			if callCount == 1 {
				return []api.ImageInfo{}, nil
			}
			// Second call is the actual test, return error
			return nil, errors.New("ECR service unavailable")
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", http.NoBody)
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "developer",
	})

	w := httptest.NewRecorder()
	router.handleListImages(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ==================== handleGetImage tests ====================

func TestHandleGetImage_Success(t *testing.T) {
	now := time.Now()
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			assert.Equal(t, "alpine:latest", image)
			return &api.ImageInfo{
				ImageID:   "img-123",
				Image:     "alpine:latest",
				CreatedBy: "user@example.com",
				CreatedAt: now,
			}, nil
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/alpine:latest", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "alpine:latest")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetImage(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.ImageInfo
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "img-123", response.ImageID)
	assert.Equal(t, "alpine:latest", response.Image)
}

func TestHandleGetImage_NotFound(t *testing.T) {
	runner := &testRunner{
		getImageFunc: func(_ string) (*api.ImageInfo, error) {
			return nil, apperrors.ErrNotFound("image not found", nil)
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/nonexistent:latest", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "nonexistent:latest")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetImage(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleGetImage_ECRImage(t *testing.T) {
	now := time.Now()
	ecrImage := "123456789.dkr.ecr.us-east-1.amazonaws.com/myapp:v1.0"
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			assert.Equal(t, ecrImage, image)
			return &api.ImageInfo{
				ImageID:   "img-ecr-123",
				Image:     ecrImage,
				CreatedBy: "user@example.com",
				CreatedAt: now,
			}, nil
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/"+ecrImage, http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", ecrImage)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetImage(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleGetImage_EmptyImagePath(t *testing.T) {
	router := newImageHandlerRouter(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/", http.NoBody)

	// Set up chi route context with empty image path
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetImage(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleGetImage_WithDigest(t *testing.T) {
	now := time.Now()
	imageWithDigest := "alpine@sha256:abc123def456"
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			assert.Equal(t, imageWithDigest, image)
			return &api.ImageInfo{
				ImageID:   "img-digest-123",
				Image:     imageWithDigest,
				CreatedBy: "user@example.com",
				CreatedAt: now,
			}, nil
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/"+imageWithDigest, http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", imageWithDigest)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetImage(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== handleRemoveImage tests ====================

func TestHandleRemoveImage_Success(t *testing.T) {
	runner := &testRunner{
		removeImageFunc: func(_ context.Context, image string) error {
			assert.Equal(t, "alpine:latest", image)
			return nil
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/alpine:latest", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "alpine:latest")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleRemoveImage(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.RemoveImageResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "alpine:latest", response.Image)
	assert.Contains(t, response.Message, "successfully")
}

func TestHandleRemoveImage_NotFound(t *testing.T) {
	runner := &testRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return apperrors.ErrNotFound("image not found", nil)
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/nonexistent:latest", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "nonexistent:latest")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleRemoveImage(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleRemoveImage_ImageInUse(t *testing.T) {
	runner := &testRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return apperrors.ErrBadRequest("image is in use by running execution", nil)
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/alpine:latest", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "alpine:latest")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleRemoveImage(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleRemoveImage_EmptyImagePath(t *testing.T) {
	router := newImageHandlerRouter(t, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/", http.NoBody)

	// Set up chi route context with empty image path
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleRemoveImage(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleRemoveImage_ECRImage(t *testing.T) {
	ecrImage := "123456789.dkr.ecr.us-east-1.amazonaws.com/myapp:v1.0"
	runner := &testRunner{
		removeImageFunc: func(_ context.Context, image string) error {
			assert.Equal(t, ecrImage, image)
			return nil
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/"+ecrImage, http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", ecrImage)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleRemoveImage(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleRemoveImage_ServiceError(t *testing.T) {
	runner := &testRunner{
		removeImageFunc: func(_ context.Context, _ string) error {
			return errors.New("ECR service unavailable")
		},
	}
	router := newImageHandlerRouter(t, runner)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/alpine:latest", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "alpine:latest")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleRemoveImage(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ==================== Benchmark tests ====================

func BenchmarkHandleRegisterImage(b *testing.B) {
	runner := &testRunner{}
	router := newImageHandlerRouter(&testing.T{}, runner)

	reqBody := api.RegisterImageRequest{
		Image: "alpine:latest",
	}
	body, _ := json.Marshal(reqBody)

	user := testutil.NewUserBuilder().
		WithEmail("user@example.com").
		WithRole("admin").
		Build()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addAuthenticatedUser(req, user)

		w := httptest.NewRecorder()
		router.handleRegisterImage(w, req)
	}
}

func BenchmarkHandleListImages(b *testing.B) {
	runner := &testRunner{
		listImagesFunc: func() ([]api.ImageInfo, error) {
			return []api.ImageInfo{}, nil
		},
	}
	router := newImageHandlerRouter(&testing.T{}, runner)

	user := testutil.NewUserBuilder().
		WithEmail("user@example.com").
		WithRole("developer").
		Build()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/images", http.NoBody)
		req = addAuthenticatedUser(req, user)

		w := httptest.NewRecorder()
		router.handleListImages(w, req)
	}
}

func BenchmarkHandleGetImage(b *testing.B) {
	now := time.Now()
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			return &api.ImageInfo{
				ImageID:   "img-123",
				Image:     image,
				CreatedBy: "user@example.com",
				CreatedAt: now,
			}, nil
		},
	}
	router := newImageHandlerRouter(&testing.T{}, runner)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/images/alpine:latest", http.NoBody)

		// Set up chi route context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("*", "alpine:latest")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		router.handleGetImage(w, req)
	}
}
