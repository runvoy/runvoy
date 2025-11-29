package infra

import (
	"fmt"
	"net/url"
)

// BuildLogsURL constructs a properly formatted URL for viewing logs in the web viewer.
// It takes a base web URL and an execution ID, and returns a URL string with the /logs path
// and execution_id query parameter properly encoded.
//
// If URL parsing or path joining fails, it falls back to simple string concatenation
// to ensure a URL is always returned (though it may not be perfectly formatted).
func BuildLogsURL(webURL, executionID string) string {
	baseURL, err := url.Parse(webURL)
	if err != nil {
		// Fallback to simple string concatenation if URL parsing fails
		return fmt.Sprintf("%s/logs?execution_id=%s", webURL, executionID)
	}

	// Join the path with /logs, handling trailing slashes properly
	joinedPath, err := url.JoinPath(baseURL.Path, "logs")
	if err != nil {
		// Fallback to simple string concatenation if path joining fails
		return fmt.Sprintf("%s/logs?execution_id=%s", webURL, executionID)
	}
	baseURL.Path = joinedPath
	baseURL.RawQuery = "execution_id=" + url.QueryEscape(executionID)

	return baseURL.String()
}
