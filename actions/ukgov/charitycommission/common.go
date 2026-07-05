package charitycommission

import (
	"context"
	"fmt"
	"net/http"

	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

// BaseURL is the Charity Commission Register API root (Azure API Management).
// Package variable so tests can point it at a mock server. Verify the exact
// base at build time via the developer portal.
var BaseURL = "https://api.charitycommission.gov.uk/register/api"

// Get performs an authenticated GET against the Charity Commission API, sending
// the Azure APIM subscription key header.
func Get(ctx context.Context, apiKey, path string) (int, []byte, error) {
	return ukgov_common.Fetch(ctx, http.MethodGet, BaseURL+path, map[string]string{
		"Ocp-Apim-Subscription-Key": apiKey,
	})
}

// StatusMessage maps a non-success status to a friendly message. 404 is handled
// per-action (its meaning is contextual).
func StatusMessage(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "Charity Commission authentication failed — check the subscription key."
	case http.StatusTooManyRequests:
		return "Charity Commission rate limit exceeded. Try again shortly."
	default:
		return fmt.Sprintf("Charity Commission returned status %d.", status)
	}
}
