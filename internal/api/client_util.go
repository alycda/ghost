package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/timescale/ghost/internal/config"
)

// Singleton HTTP client with a 30 second request timeout
var HTTPClient = &http.Client{
	Timeout: 30 * time.Second, // Overall request timeout
}

// AuthMethod configures the Authorization header on outgoing API requests.
type AuthMethod func(req *http.Request)

// APIKeyAuth returns an AuthMethod that authenticates using a Bearer token.
// Use this for new-style API keys (format: gt_<access_key>_<secret_key>).
func APIKeyAuth(apiKey string) AuthMethod {
	return func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

// LegacyAPIKeyAuth returns an AuthMethod that authenticates with an API key using HTTP Basic auth.
// Use this for legacy API keys stored in credentials (access_key:secret_key encoded as Basic auth).
func LegacyAPIKeyAuth(apiKey string) AuthMethod {
	return func(req *http.Request) {
		encodedKey := base64.StdEncoding.EncodeToString([]byte(apiKey))
		req.Header.Set("Authorization", "Basic "+encodedKey)
	}
}

// TokenAuth returns an AuthMethod that authenticates using an OAuth2 token.
func TokenAuth(token *oauth2.Token) AuthMethod {
	return func(req *http.Request) {
		token.SetAuthHeader(req)
	}
}

// NewGhostClient creates a new API client with the given API URL and auth method.
// Declared as a var so tests can replace it.
var NewGhostClient = func(apiURL string, auth AuthMethod) (ClientWithResponsesInterface, error) {
	client, err := NewClientWithResponses(
		apiURL,
		WithHTTPClient(HTTPClient),
		WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			auth(req)
			req.Header.Set("User-Agent", fmt.Sprintf("ghost-cli/%s", config.Version))
			return nil
		}))

	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return client, nil
}

// Error implements the error interface for the Error type.
// This allows Error values to be used directly as Go errors.
func (e *Error) Error() string {
	if e == nil {
		return "unknown error"
	}
	if e.Message != "" {
		return e.Message
	}
	return "unknown error"
}

// Database returns the plain Database inside a DatabaseWithUsage. One place
// for the conversion, so a field added to the schema cannot be dropped by a
// hand-written copy: the function-tool manager did exactly that with dbname.
func (d DatabaseWithUsage) Database() Database {
	return Database{
		Dbname:     d.Dbname,
		Host:       d.Host,
		ID:         d.ID,
		Name:       d.Name,
		Password:   d.Password,
		Port:       d.Port,
		Size:       d.Size,
		Status:     d.Status,
		StorageMib: d.StorageMib,
		Type:       d.Type,
	}
}

// WithUsage is the inverse, for a server that lists databases.
func (d Database) WithUsage(computeMinutes *int64) DatabaseWithUsage {
	return DatabaseWithUsage{
		ComputeMinutes: computeMinutes,
		Dbname:         d.Dbname,
		Host:           d.Host,
		ID:             d.ID,
		Name:           d.Name,
		Password:       d.Password,
		Port:           d.Port,
		Size:           d.Size,
		Status:         d.Status,
		StorageMib:     d.StorageMib,
		Type:           d.Type,
	}
}
