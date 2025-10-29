package lambdaapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/go-chi/chi/v5"
)

// ChiRouterToLambdaAdapter adapts a chi router to work with Lambda Function URL events
func ChiRouterToLambdaAdapter(router *chi.Mux) func(context.Context, events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	return func(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
		// Ensure Lambda context is available in the request context
		// This is important for the request ID middleware to work properly
		ctx = lambdacontext.NewContext(ctx, &lambdacontext.LambdaContext{
			AwsRequestID: req.RequestContext.RequestID,
		})

		httpReq := lambdaRequestToHTTPRequest(ctx, req)
		if httpReq != nil {
			httpReq.RemoteAddr = req.RequestContext.HTTP.SourceIP
		}

		var buf bytes.Buffer
		responseWriter := &lambdaResponseWriter{
			buffer: &buf,
			header: make(http.Header),
			status: http.StatusOK,
		}

		router.ServeHTTP(responseWriter, httpReq)

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
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err == nil {
			body = string(decoded)
		}
	}

	httpReq, _ := http.NewRequestWithContext(ctx, req.RequestContext.HTTP.Method, url, strings.NewReader(body))

	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	httpReq.URL.RawQuery = req.RawQueryString

	return httpReq
}

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
	headers := make(map[string]string)
	for key, values := range w.header {
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
