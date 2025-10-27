package aws

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// LambdaResponseWriter implements http.ResponseWriter for Lambda
type LambdaResponseWriter struct {
	statusCode int
	headers    map[string]string
	body       bytes.Buffer
}

// NewLambdaResponseWriter creates a new Lambda response writer
func NewLambdaResponseWriter() *LambdaResponseWriter {
	return &LambdaResponseWriter{
		statusCode: 200,
		headers:    make(map[string]string),
	}
}

// Header returns the header map
func (w *LambdaResponseWriter) Header() http.Header {
	// Convert map[string]string to http.Header
	h := make(http.Header)
	for k, v := range w.headers {
		h.Set(k, v)
	}
	return h
}

// Write writes data to the response body
func (w *LambdaResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

// WriteHeader sets the status code
func (w *LambdaResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// ToLambdaResponse converts to Lambda response format
func (w *LambdaResponseWriter) ToLambdaResponse() events.LambdaFunctionURLResponse {
	return events.LambdaFunctionURLResponse{
		StatusCode: w.statusCode,
		Headers:    w.headers,
		Body:       w.body.String(),
	}
}

// LambdaToHTTPRequest converts a Lambda request to an HTTP request
func LambdaToHTTPRequest(request events.LambdaFunctionURLRequest) (*http.Request, error) {
	// Create a new HTTP request
	req, err := http.NewRequest(request.RequestContext.HTTP.Method, request.RawPath, strings.NewReader(request.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}

	// Set query parameters
	for key, value := range request.QueryStringParameters {
		req.URL.Query().Add(key, value)
	}

	return req, nil
}