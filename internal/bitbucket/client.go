package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is the internal HTTP transport for Bitbucket API communication.
type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

// ErrorResponse is returned when the Bitbucket API responds with a non-2xx status.
type ErrorResponse struct {
	StatusCode int
	Body       string
}

// Error returns the formatted API error text.
func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("bitbucket api request failed with status %d: %s", e.StatusCode, e.Body)
}

// NewClient creates an internal transport client with a validated base URL.
//
// If httpClient is nil, a default client with a 30-second timeout is used.
func NewClient(baseURL, token string, httpClient *http.Client) (*Client, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("invalid base url: %w", err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid base url: %q", baseURL)
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{
		baseURL:    parsedURL,
		token:      token,
		httpClient: httpClient,
	}, nil
}

// GetJSON performs a GET request and decodes a JSON response into T.
func GetJSON[T any](ctx context.Context, client *Client, path string) (T, error) {
	var zero T

	request, err := client.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return zero, err
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return zero, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return zero, parseErrorResponse(response)
	}

	if err := json.NewDecoder(response.Body).Decode(&zero); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}

	return zero, nil
}

// PostJSON performs a POST request with a JSON payload and decodes a JSON response into T.
func PostJSON[T any](ctx context.Context, client *Client, path string, payload any) (T, error) {
	var zero T

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return zero, fmt.Errorf("encode request: %w", err)
	}

	request, err := client.newRequest(ctx, http.MethodPost, path, bytes.NewReader(bodyBytes))
	if err != nil {
		return zero, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return zero, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return zero, parseErrorResponse(response)
	}

	if err := json.NewDecoder(response.Body).Decode(&zero); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}

	return zero, nil
}

// newRequest builds an authenticated HTTP request against the configured base URL.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	relative, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	fullURL := c.baseURL.ResolveReference(relative)
	request, err := http.NewRequestWithContext(ctx, method, fullURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	return request, nil
}

// parseErrorResponse reads and normalizes API error responses.
func parseErrorResponse(response *http.Response) error {
	bodyBytes, err := io.ReadAll(io.LimitReader(response.Body, 8*1024))
	if err != nil {
		return &ErrorResponse{StatusCode: response.StatusCode, Body: http.StatusText(response.StatusCode)}
	}

	bodyText := strings.TrimSpace(string(bodyBytes))
	if bodyText == "" {
		bodyText = http.StatusText(response.StatusCode)
	}

	return &ErrorResponse{StatusCode: response.StatusCode, Body: bodyText}
}
