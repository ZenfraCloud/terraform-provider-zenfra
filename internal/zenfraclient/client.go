// ABOUTME: Core HTTP client for the Zenfra REST API.
// ABOUTME: Provides NewClient constructor, doRequest/doJSON helpers with retry and auth handling.

package zenfraclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout   = 30 * time.Second
	defaultUserAgent = "terraform-provider-zenfra/0.1.0"
)

// ClientConfig holds configuration for creating a new Client.
type ClientConfig struct {
	Endpoint   string        // Required: Zenfra API base URL (e.g., "https://api.zenfra.io")
	APIToken   string        // Required: Bearer token for authentication
	UserAgent  string        // Optional: defaults to "terraform-provider-zenfra/<version>"
	Timeout    time.Duration // Optional: HTTP client timeout, defaults to 30s
	MaxRetries int           // Optional: max retry attempts, defaults to 3
}

// Client is the Zenfra API client.
type Client struct {
	baseURL    string
	apiToken   string
	userAgent  string
	httpClient *http.Client
	retry      retryConfig
}

// NewClient creates a new Zenfra API client.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("api_token is required")
	}

	userAgent := cfg.UserAgent
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	retryCfg := defaultRetryConfig()
	if cfg.MaxRetries > 0 {
		retryCfg.maxRetries = cfg.MaxRetries
	}

	return &Client{
		baseURL:   strings.TrimRight(cfg.Endpoint, "/"),
		apiToken:  cfg.APIToken,
		userAgent: userAgent,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retry: retryCfg,
	}, nil
}

// doRequest executes an HTTP request with retry logic and returns the raw response.
// The caller is responsible for closing the response body.
//
//nolint:gocognit,gocyclo // retry loop with error handling is inherently complex
func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	url := c.baseURL + path

	var lastResp *http.Response
	var lastErr error

	for attempt := range c.retry.maxRetries + 1 {
		// Re-create the body reader for retries.
		if attempt > 0 && body != nil {
			jsonBytes, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshaling request body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.apiToken)
		req.Header.Set("User-Agent", c.userAgent)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("executing request: %w", err)
			if ctx.Err() != nil {
				return nil, lastErr
			}
			// Network errors are retryable.
			if attempt < c.retry.maxRetries {
				if sleepErr := sleepWithContext(ctx, retryDelay(c.retry, attempt, nil)); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			return nil, lastErr
		}

		if !isRetryableStatus(resp.StatusCode) {
			return resp, nil
		}

		// Close body before retry.
		lastResp = resp
		_ = resp.Body.Close()

		if attempt < c.retry.maxRetries {
			if sleepErr := sleepWithContext(ctx, retryDelay(c.retry, attempt, resp)); sleepErr != nil {
				return nil, sleepErr
			}
		}
	}

	// All retries exhausted.
	if lastResp != nil {
		return nil, &APIError{
			StatusCode: lastResp.StatusCode,
			Message:    fmt.Sprintf("request failed after %d retries with status %d", c.retry.maxRetries, lastResp.StatusCode),
		}
	}
	return nil, lastErr
}

// doJSON executes an HTTP request, parses the JSON response into result.
// If result is nil, only the status code is checked.
func (c *Client) doJSON(ctx context.Context, method, path string, body, result any) error {
	resp, err := c.doRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if err := checkResponse(resp); err != nil {
		return err
	}

	if result == nil {
		return nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("unmarshaling response (status %d): %w", resp.StatusCode, err)
	}

	return nil
}

// checkResponse inspects the HTTP response and returns a typed error for non-2xx status codes.
func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)

	apiErr := APIError{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-ID"),
	}

	// Try to parse error message from response body.
	var errBody struct {
		Error   string            `json:"error"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields,omitempty"`
	}
	if json.Unmarshal(bodyBytes, &errBody) == nil {
		if errBody.Message != "" {
			apiErr.Message = errBody.Message
		} else if errBody.Error != "" {
			apiErr.Message = errBody.Error
		}
	}

	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(resp.StatusCode)
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return &NotFoundError{APIError: apiErr}
	case http.StatusConflict:
		return &ConflictError{APIError: apiErr}
	case http.StatusUnauthorized:
		return &UnauthorizedError{APIError: apiErr}
	case http.StatusForbidden:
		return &ForbiddenError{APIError: apiErr}
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return &ValidationError{APIError: apiErr, Fields: errBody.Fields}
	default:
		return &apiErr
	}
}
