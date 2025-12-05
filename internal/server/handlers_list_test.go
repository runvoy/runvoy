package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/runvoy/runvoy/internal/backend/orchestrator"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
)

func TestHandleListWithAuth_Success(t *testing.T) {
	router := &Router{svc: &orchestrator.Service{Logger: testutil.SilentLogger()}}
	rr := httptest.NewRecorder()

	router.handleListWithAuth(rr, httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody), func() (any, error) {
		return map[string]string{"status": "ok"}, nil
	}, "list resources")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"ok"}`, rr.Body.String())
}

func TestHandleListWithAuth_MarshalError(t *testing.T) {
	router := &Router{svc: &orchestrator.Service{Logger: testutil.SilentLogger()}}
	rr := httptest.NewRecorder()

	router.handleListWithAuth(rr, httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody), func() (any, error) {
		return make(chan int), nil
	}, "list resources")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.JSONEq(t, `{"error":"failed to list resources","details":"could not encode response"}`, rr.Body.String())
}
