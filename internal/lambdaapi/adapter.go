package lambdaapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/go-chi/chi/v5"
)

// ChiRouterToLambdaAdapter adapts a chi router to work with Lambda Function URL events
func ChiRouterToLambdaAdapter(router *chi.Mux) func(context.Context, events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	return func(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
		// Create HTTP request from Lambda event
		httpReq := lambdaRequestToHTTPRequest(ctx, req)

		// Create response writer
		var buf bytes.Buffer
		responseWriter := &lambdaResponseWriter{
			buffer: &buf,
			header: make(http.Header),
			status: http.StatusOK,
		}

		// Serve the request using chi router
		router.ServeHTTP(responseWriter, httpReq)

		// Build Lambda response from HTTP response
		return httpResponseToLambdaResponse(responseWriter), nil
	}
}

// lambdaRequestToHTTPRequest converts a Lambda Function URL request to an http.Request
func lambdaRequestToHTTPRequest(ctx context.Context, req events.LambdaFunctionURLRequest) *http.Request {
	url := "https://" + req.RequestContext.DomainName
	if req.RawPath != "" {
		url = url + req.RawPath
	} else {
		url = url + "/"
	}
	if req.RawQueryString != "" {
		url = url + "?" + req.RawQueryString
	}

	body := req.Body
	// Check if the body is base64 encoded
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err == nil {
			body = string(decoded)
		}
	}

	httpReq, _ := http.NewRequestWithContext(ctx, req.RequestContext.HTTP.Method, url, strings.NewReader(body))

	// Copy headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Copy query parameters
	httpReq.URL.RawQuery = req.RawQueryString

	return httpReq
}

// lambdaResponseWriter is a response writer for chi that captures the response
type lambdaResponseWriter struct {
	buffer *bytes.Buffer
	header http.Header
	status int
}

func (w *lambdaResponseWriter) Header() http.Header {
	return w.header
}

func (w *lambdaResponseWriter) Write(b []byte) (int, error) {
	return w.buffer.Write(b)
}

func (w *lambdaResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

// httpResponseToLambdaResponse converts an http response to a Lambda response
func httpResponseToLambdaResponse(w *lambdaResponseWriter) events.LambdaFunctionURLResponse {
	// Convert header map to proper format (lowercase keys for Lambda)
	headers := make(map[string]string)
	for key, values := range w.header {
		// Join multiple values with comma
		if len(values) > 0 {
			headers[strings.ToLower(key)] = strings.Join(values, ",")
		}
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: w.status,
		Headers:    headers,
		Body:       w.buffer.String(),
	}
}
