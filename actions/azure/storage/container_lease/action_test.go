package azure_storage_container_lease

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

const testKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

const (
	theLeaseID = "8b1c6a2e-0f9d-4a3b-9c5e-7d2f1a4b6c8d"
	proposedID = "1f4e9c7a-3b2d-4e6f-8a1c-5d9b0e2f7a3c"
)

func baseInputs(endpoint string, extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
		{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
	}
	return append(inputs, extra...)
}

func str(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: v}
}

func num(name string, v int) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: v}
}

type capture struct {
	method, path, query string
	headers             http.Header
}

func serve(t *testing.T, status int, respHeaders map[string]string) (*httptest.Server, *capture) {
	t.Helper()
	c := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.method, c.path, c.query = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		c.headers = r.Header.Clone()
		for k, v := range respHeaders {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

// result pairs Execute's two return values so they can travel through one
// argument: Go permits the f(g()) form only when g supplies EVERY one of f's
// parameters, and mustOK also needs the *testing.T.
type result struct {
	out map[string]interface{}
	err error
}

func res(out map[string]interface{}, err error) result { return result{out, err} }

func mustOK(t *testing.T, r result) map[string]interface{} {
	t.Helper()
	if r.err != nil {
		t.Fatalf("Execute: hard error %v", r.err)
	}
	if r.out["success"] != true {
		t.Fatalf("soft failure: %v", r.out["error"])
	}
	return r.out
}

// TestAcquireLeasesTheContainer — restype=container is the whole difference
// between this action and Lease Blob: the same comp=lease against the same
// path leases a BLOB unless the resource type says otherwise, so dropping it
// would silently lease the wrong thing (or 404).
func TestAcquireLeasesTheContainer(t *testing.T) {
	srv, c := serve(t, http.StatusCreated, map[string]string{"x-ms-lease-id": theLeaseID})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "acquire"),
		num("duration", 30),
	))))

	if c.method != http.MethodPut || c.path != "/my-container" {
		t.Errorf("request = %s %s, want PUT /my-container", c.method, c.path)
	}
	if !strings.Contains(c.query, "restype=container") || !strings.Contains(c.query, "comp=lease") {
		t.Errorf("query = %q, want both restype=container and comp=lease", c.query)
	}
	if got := c.headers.Get("x-ms-lease-action"); got != "acquire" {
		t.Errorf("x-ms-lease-action = %q", got)
	}
	if got := c.headers.Get("x-ms-lease-duration"); got != "30" {
		t.Errorf("x-ms-lease-duration = %q", got)
	}
	if out["lease_id"] != theLeaseID {
		t.Errorf("lease_id = %v", out["lease_id"])
	}
	if out["id"] != "my-container" {
		t.Errorf("id = %v, want the container name", out["id"])
	}
	// The summary names the container AS a container — "Acquired a 30s lease
	// on my-container" would read as a blob.
	if out["tool_result"] != "Acquired a 30s lease on container my-container" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

func TestAcquireInfiniteLease(t *testing.T) {
	srv, c := serve(t, http.StatusCreated, map[string]string{"x-ms-lease-id": theLeaseID})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "acquire"),
		num("duration", -1),
	))))

	if got := c.headers.Get("x-ms-lease-duration"); got != "-1" {
		t.Errorf("x-ms-lease-duration = %q", got)
	}
	if out["tool_result"] != "Acquired an infinite lease on container my-container" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

func TestAcquireDefaultsToSixtySeconds(t *testing.T) {
	srv, c := serve(t, http.StatusCreated, map[string]string{"x-ms-lease-id": theLeaseID})

	mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL, str("lease_action", "acquire")))))

	if got := c.headers.Get("x-ms-lease-duration"); got != "60" {
		t.Errorf("x-ms-lease-duration = %q, want the declared default of 60", got)
	}
}

func TestRenewSendsTheHeldID(t *testing.T) {
	srv, c := serve(t, http.StatusOK, map[string]string{"x-ms-lease-id": theLeaseID})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "renew"),
		str("lease_id", theLeaseID),
	))))

	if got := c.headers.Get("x-ms-lease-id"); got != theLeaseID {
		t.Errorf("x-ms-lease-id = %q", got)
	}
	if out["tool_result"] != "Renewed the lease on container my-container" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

func TestChangeSwapsTheID(t *testing.T) {
	srv, c := serve(t, http.StatusOK, map[string]string{"x-ms-lease-id": proposedID})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "change"),
		str("lease_id", theLeaseID),
		str("proposed_lease_id", proposedID),
	))))

	if got := c.headers.Get("x-ms-lease-id"); got != theLeaseID {
		t.Errorf("x-ms-lease-id = %q, want the current id", got)
	}
	if got := c.headers.Get("x-ms-proposed-lease-id"); got != proposedID {
		t.Errorf("x-ms-proposed-lease-id = %q, want the new id", got)
	}
	if out["lease_id"] != proposedID {
		t.Errorf("lease_id = %v", out["lease_id"])
	}
	if out["tool_result"] != "Changed the lease ID on container my-container" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

func TestReleaseSendsTheHeldID(t *testing.T) {
	srv, c := serve(t, http.StatusOK, nil)

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "release"),
		str("lease_id", theLeaseID),
	))))

	if got := c.headers.Get("x-ms-lease-action"); got != "release" {
		t.Errorf("x-ms-lease-action = %q", got)
	}
	if got := c.headers.Get("x-ms-lease-id"); got != theLeaseID {
		t.Errorf("x-ms-lease-id = %q", got)
	}
	if out["lease_id"] != "" {
		t.Errorf("lease_id = %v, want empty after a release", out["lease_id"])
	}
	if out["tool_result"] != "Released the lease on container my-container" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

func TestBreakReportsTimeRemaining(t *testing.T) {
	srv, c := serve(t, http.StatusAccepted, map[string]string{"x-ms-lease-time": "25"})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "break"),
		num("break_period", 30),
	))))

	if got := c.headers.Get("x-ms-lease-break-period"); got != "30" {
		t.Errorf("x-ms-lease-break-period = %q", got)
	}
	if _, ok := c.headers["X-Ms-Lease-Id"]; ok {
		t.Errorf("break sent x-ms-lease-id with none supplied")
	}
	if out["lease_time"] != 25 {
		t.Errorf("lease_time = %v, want 25", out["lease_time"])
	}
	if out["tool_result"] != "Broke the lease on container my-container — it ends in 25s" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

func TestValidationFailuresAreSoftErrors(t *testing.T) {
	cases := []struct {
		name  string
		extra []*core.Connection
		want  string
	}{
		{"no action", nil, "lease_action is required"},
		{"unknown action", []*core.Connection{str("lease_action", "steal")}, `lease_action "steal" is not supported`},
		{"renew without id", []*core.Connection{str("lease_action", "renew")}, "lease_id is required"},
		{"release without id", []*core.Connection{str("lease_action", "release")}, "lease_id is required"},
		{"change without id", []*core.Connection{str("lease_action", "change")}, "lease_id is required"},
		{"change without proposal", []*core.Connection{str("lease_action", "change"), str("lease_id", theLeaseID)}, "proposed_lease_id is required"},
		{"bad duration", []*core.Connection{str("lease_action", "acquire"), num("duration", 5)}, "duration must be between 15 and 60 seconds"},
		{"bad break period", []*core.Connection{str("lease_action", "break"), num("break_period", 90)}, "break_period must be between 0 and 60 seconds"},
		{"non-GUID proposal", []*core.Connection{str("lease_action", "acquire"), str("proposed_lease_id", "lease-1")}, "proposed_lease_id must be a GUID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid", tc.extra...))
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), tc.want) {
				t.Errorf("out = %v, want a soft failure containing %q", out, tc.want)
			}
		})
	}
}

func TestMissingContainerIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: "http://unused.invalid"},
		str("lease_action", "acquire"),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "container is required") {
		t.Errorf("out = %v", out)
	}
}

// TestLeasedContainerErrorIsSoft — a container lease guards the container, so
// a second acquire is a 409 an operator will meet routinely.
func TestLeasedContainerErrorIsSoft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>LeaseAlreadyPresent</Code><Message>There is already a lease present.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "acquire"),
		num("duration", 30),
	))
	if err != nil {
		t.Fatalf("Execute: hard error %v — a service refusal is data, not a crash", err)
	}
	msg, _ := out["error"].(string)
	if out["success"] != false || !strings.Contains(msg, "LeaseAlreadyPresent: There is already a lease present.") {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}
