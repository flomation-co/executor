package awx

import (
	"net/http"
	"strings"
	"testing"
)

// ★ AWX refuses to DELETE an object while a job that ran against it is still
// flushing its events to Postgres — and reports that as a 403, in the same shape
// as a permissions denial. It is neither permanent nor a permissions problem.
//
// Live-verified on AWX 24.6.1 (2026-07-14): deleting a project the instant its
// source-control sync reported `successful` is refused with exactly this body;
// the identical delete succeeds a second or two later.
func TestCheckResponseProcessingEvents403(t *testing.T) {
	resp := &Response{
		StatusCode: http.StatusForbidden,
		Method:     http.MethodDelete,
		URL:        "http://awx.example.com/api/v2/projects/85/",
		Body:       []byte(`{"detail":"Related job project_update 85 (successful) is still processing events."}`),
	}

	// A realistic token: Redact is a plain substring replace with no minimum
	// length (deliberately — see its doc comment), so a one-character token would
	// scrub every "t" out of the message and tell us nothing.
	err := CheckResponse(Auth{Method: AuthMethodToken, Token: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"}, resp)
	if err == nil {
		t.Fatal("expected an error for a 403")
	}
	msg := err.Error()

	// It must be reported as temporary and retryable...
	for _, want := range []string{"TEMPORARY", "wait a few seconds", "still processing events"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the message does not contain %q: %s", want, msg)
		}
	}
	// ...and must NOT send the operator hunting for a permission they already have.
	if strings.Contains(msg, "does not have permission") {
		t.Errorf("a transient 403 was reported as a permissions failure: %s", msg)
	}
}

// A 403 that really IS a permissions problem must keep its old wording — the new
// branch must not swallow every 403.
func TestCheckResponsePlain403StillPermissions(t *testing.T) {
	auth := Auth{Method: AuthMethodToken, Token: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"}

	// DRF's own permission denial, which always carries a detail.
	withDetail := &Response{
		StatusCode: http.StatusForbidden,
		Method:     http.MethodDelete,
		URL:        "http://awx.example.com/api/v2/projects/85/",
		Body:       []byte(`{"detail":"You do not have permission to perform this action."}`),
	}
	err := CheckResponse(auth, withDetail)
	if err == nil || strings.Contains(err.Error(), "TEMPORARY") {
		t.Fatalf("a real permissions 403 was mistaken for the event-flush one: %v", err)
	}
	if !strings.Contains(err.Error(), "You do not have permission") {
		t.Fatalf("the permission detail was lost: %v", err)
	}

	// A bodiless 403.
	bare := &Response{
		StatusCode: http.StatusForbidden,
		Method:     http.MethodDelete,
		URL:        "http://awx.example.com/api/v2/projects/85/",
	}
	err = CheckResponse(auth, bare)
	if err == nil || !strings.Contains(err.Error(), "does not have permission") {
		t.Fatalf("a bare 403 should still read as a permissions failure, got: %v", err)
	}
}

// The launch 403 (a read-scoped token) must not be swallowed by the new branch.
func TestCheckResponseLaunch403Unchanged(t *testing.T) {
	resp := &Response{
		StatusCode: http.StatusForbidden,
		Method:     http.MethodPost,
		URL:        "http://awx.example.com/api/v2/job_templates/7/launch/",
		Body:       []byte(`{"detail":"You do not have permission to perform this action."}`),
	}
	// A realistic token: Redact is a plain substring replace with no minimum
	// length (deliberately — see its doc comment), so a one-character token would
	// scrub every "t" out of the message and tell us nothing.
	err := CheckResponse(Auth{Method: AuthMethodToken, Token: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"}, resp)
	if err == nil || !strings.Contains(err.Error(), "READ-scoped") {
		t.Fatalf("the launch 403 wording was lost, got: %v", err)
	}
}
