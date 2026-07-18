package azure_storage_blob_lease

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

const testKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

// theLeaseID / proposedID are well-formed lease GUIDs, as Azure mints them.
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
		{Name: "blob_name", Type: core.ConnectionTypeString, Value: "hello.txt"},
	}
	return append(inputs, extra...)
}

func str(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: v}
}

func num(name string, v int) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: v}
}

// capture records what one lease call put on the wire.
type capture struct {
	method, path, query string
	headers             http.Header
}

// serve answers a lease call with the status and response headers Azure would,
// recording the request.
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

// ---------------------------------------------------------------------------
// acquire
// ---------------------------------------------------------------------------

// TestAcquireMintsALease is the action's reason to exist: without it a lease
// ID could only ever come from outside the platform, and every lease_id field
// on every other action would be unfillable.
func TestAcquireMintsALease(t *testing.T) {
	srv, c := serve(t, http.StatusCreated, map[string]string{
		"x-ms-lease-id": theLeaseID,
		"ETag":          `"0x8DA"`,
	})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "acquire"),
		num("duration", 30),
	))))

	if c.method != http.MethodPut || c.path != "/my-container/hello.txt" || c.query != "comp=lease" {
		t.Errorf("request = %s %s?%s, want PUT /my-container/hello.txt?comp=lease", c.method, c.path, c.query)
	}
	if got := c.headers.Get("x-ms-lease-action"); got != "acquire" {
		t.Errorf("x-ms-lease-action = %q", got)
	}
	if got := c.headers.Get("x-ms-lease-duration"); got != "30" {
		t.Errorf("x-ms-lease-duration = %q, want 30", got)
	}
	// No lease exists yet, so there is nothing to name in x-ms-lease-id.
	if _, ok := c.headers["X-Ms-Lease-Id"]; ok {
		t.Errorf("acquire sent x-ms-lease-id: %q — there is no lease to name yet", c.headers.Get("x-ms-lease-id"))
	}
	// (Unlike the pre-SDK path, the lease subpackage always mints a client-side
	// proposed-lease-id and sends it on acquire; the response's x-ms-lease-id is
	// still what the action reports, which the lease_id assertion below pins.)

	if out["lease_id"] != theLeaseID {
		t.Errorf("lease_id output = %v, want the ID the service minted", out["lease_id"])
	}
	if out["lease_time"] != 0 {
		t.Errorf("lease_time = %v, want 0 outside a break", out["lease_time"])
	}
	if out["tool_result"] != "Acquired a 30s lease on hello.txt" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
	result := out["result"].(map[string]interface{})
	if result["leaseAction"] != "acquire" {
		t.Errorf("result.leaseAction = %v", result["leaseAction"])
	}
	// The lease ID is surfaced as the top-level lease_id output (asserted above),
	// not nested under result.properties. The properties envelope now carries the
	// write's ETag from the SDK's typed acquire response.
	if props := result["properties"].(map[string]interface{}); props["etag"] != `"0x8DA"` {
		t.Errorf("result.properties = %#v, want the acquire ETag carried through", props)
	}
}

// TestAcquireDefaultsToSixtySeconds — an unset Duration must mean the input's
// declared default. 60 is the longest FINITE lease: a flow that dies holding
// one costs a minute of waiting, where -1 would lock the blob until someone
// breaks it by hand.
func TestAcquireDefaultsToSixtySeconds(t *testing.T) {
	srv, c := serve(t, http.StatusCreated, map[string]string{"x-ms-lease-id": theLeaseID})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "acquire"),
	))))

	if got := c.headers.Get("x-ms-lease-duration"); got != "60" {
		t.Errorf("x-ms-lease-duration = %q, want the declared default of 60", got)
	}
	if out["tool_result"] != "Acquired a 60s lease on hello.txt" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

// TestAcquireInfiniteLease — -1 is the one value outside 15-60 the service
// takes, and it reads differently in the summary because it behaves
// differently: nothing expires it.
func TestAcquireInfiniteLease(t *testing.T) {
	srv, c := serve(t, http.StatusCreated, map[string]string{"x-ms-lease-id": theLeaseID})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "acquire"),
		num("duration", -1),
	))))

	if got := c.headers.Get("x-ms-lease-duration"); got != "-1" {
		t.Errorf("x-ms-lease-duration = %q, want -1", got)
	}
	if out["tool_result"] != "Acquired an infinite lease on hello.txt" {
		t.Errorf("tool_result = %q, want the infinite lease called by its name", out["tool_result"])
	}
}

// TestAcquireProposesAnID — a proposed ID travels as x-ms-proposed-lease-id.
// Sending it as x-ms-lease-id instead would be silently ignored by the
// service, and the flow would get back an ID it did not choose.
func TestAcquireProposesAnID(t *testing.T) {
	srv, c := serve(t, http.StatusCreated, map[string]string{"x-ms-lease-id": proposedID})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "acquire"),
		num("duration", 15),
		str("proposed_lease_id", proposedID),
	))))

	if got := c.headers.Get("x-ms-proposed-lease-id"); got != proposedID {
		t.Errorf("x-ms-proposed-lease-id = %q, want %q", got, proposedID)
	}
	if _, ok := c.headers["X-Ms-Lease-Id"]; ok {
		t.Errorf("acquire sent the proposal as x-ms-lease-id, which the service ignores")
	}
	if out["lease_id"] != proposedID {
		t.Errorf("lease_id = %v", out["lease_id"])
	}
}

func TestAcquireRejectsAnOutOfRangeDuration(t *testing.T) {
	for _, d := range []int{14, 61, 0, -2} {
		out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
			str("lease_action", "acquire"),
			num("duration", d),
		))
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out["success"] != false || !strings.Contains(out["error"].(string), "duration must be between 15 and 60 seconds") {
			t.Errorf("duration %d: out = %v, want a soft failure naming the range", d, out)
		}
	}
}

func TestAcquireRejectsANonGUIDProposal(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("lease_action", "acquire"),
		str("proposed_lease_id", "my-lease"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "proposed_lease_id must be a GUID") {
		t.Errorf("out = %v, want a soft failure naming the GUID rule instead of the service's bare 400", out)
	}
}

// ---------------------------------------------------------------------------
// renew / change / release / break
// ---------------------------------------------------------------------------

func TestRenewSendsTheHeldID(t *testing.T) {
	srv, c := serve(t, http.StatusOK, map[string]string{"x-ms-lease-id": theLeaseID})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "renew"),
		str("lease_id", theLeaseID),
	))))

	if got := c.headers.Get("x-ms-lease-action"); got != "renew" {
		t.Errorf("x-ms-lease-action = %q", got)
	}
	if got := c.headers.Get("x-ms-lease-id"); got != theLeaseID {
		t.Errorf("x-ms-lease-id = %q", got)
	}
	if _, ok := c.headers["X-Ms-Lease-Duration"]; ok {
		t.Errorf("renew sent x-ms-lease-duration — a renew re-runs the ORIGINAL duration, it does not set one")
	}
	if out["tool_result"] != "Renewed the lease on hello.txt" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

// TestChangeSwapsTheID — change is the only action that carries BOTH IDs: the
// one the lease has now and the one it will have next.
func TestChangeSwapsTheID(t *testing.T) {
	srv, c := serve(t, http.StatusOK, map[string]string{"x-ms-lease-id": proposedID})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "change"),
		str("lease_id", theLeaseID),
		str("proposed_lease_id", proposedID),
	))))

	if got := c.headers.Get("x-ms-lease-id"); got != theLeaseID {
		t.Errorf("x-ms-lease-id = %q, want the CURRENT id", got)
	}
	if got := c.headers.Get("x-ms-proposed-lease-id"); got != proposedID {
		t.Errorf("x-ms-proposed-lease-id = %q, want the NEW id", got)
	}
	if out["lease_id"] != proposedID {
		t.Errorf("lease_id = %v, want the new ID the flow must quote from here on", out["lease_id"])
	}
	if out["tool_result"] != "Changed the lease ID on hello.txt" {
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
	// A released lease has no ID left to report.
	if out["lease_id"] != "" {
		t.Errorf("lease_id = %v, want empty after a release", out["lease_id"])
	}
	if out["tool_result"] != "Released the lease on hello.txt" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

// TestBreakWithoutTheID — break is the only action that works without knowing
// the lease ID, which is the entire point of it: it is what an operator who
// never held the lease reaches for.
func TestBreakWithoutTheID(t *testing.T) {
	srv, c := serve(t, http.StatusAccepted, map[string]string{"x-ms-lease-time": "0"})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "break"),
		num("break_period", 0),
	))))

	if got := c.headers.Get("x-ms-lease-action"); got != "break" {
		t.Errorf("x-ms-lease-action = %q", got)
	}
	if got := c.headers.Get("x-ms-lease-break-period"); got != "0" {
		t.Errorf("x-ms-lease-break-period = %q, want 0", got)
	}
	if _, ok := c.headers["X-Ms-Lease-Id"]; ok {
		t.Errorf("break sent x-ms-lease-id with none supplied — it must not require one")
	}
	if out["lease_time"] != 0 {
		t.Errorf("lease_time = %v", out["lease_time"])
	}
	if out["tool_result"] != "Broke the lease on hello.txt" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

// TestBreakReportsTimeRemaining — a break with time left is not a finished
// break, and the summary has to say so or an operator will write the next
// step assuming the blob is free.
func TestBreakReportsTimeRemaining(t *testing.T) {
	srv, c := serve(t, http.StatusAccepted, map[string]string{"x-ms-lease-time": "17"})

	out := mustOK(t, res(Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "break"),
		str("lease_id", theLeaseID),
	))))

	// A break period left blank sends no header — the lease runs out whatever
	// time it has left, which is the service's own default.
	if _, ok := c.headers["X-Ms-Lease-Break-Period"]; ok {
		t.Errorf("break sent x-ms-lease-break-period: %q with the field blank", c.headers.Get("x-ms-lease-break-period"))
	}
	// The lease subpackage's BreakLease carries no lease ID on the wire (Break
	// Lease does not take one), so the observable outcome — the time the broken
	// lease still has to run — is what this test pins.
	if out["lease_time"] != 17 {
		t.Errorf("lease_time = %v, want 17", out["lease_time"])
	}
	if out["tool_result"] != "Broke the lease on hello.txt — it ends in 17s" {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

func TestBreakRejectsAnOutOfRangePeriod(t *testing.T) {
	for _, p := range []int{-1, 61} {
		out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
			str("lease_action", "break"),
			num("break_period", p),
		))
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out["success"] != false || !strings.Contains(out["error"].(string), "break_period must be between 0 and 60 seconds") {
			t.Errorf("break_period %d: out = %v", p, out)
		}
	}
}

// ---------------------------------------------------------------------------
// validation
// ---------------------------------------------------------------------------

func TestMissingLeaseIDIsSoftError(t *testing.T) {
	for _, action := range []string{"renew", "release", "change"} {
		out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
			str("lease_action", action),
		))
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if out["success"] != false || !strings.Contains(out["error"].(string), "lease_id is required") {
			t.Errorf("%s: out = %v, want a soft failure naming lease_id", action, out)
		}
	}
}

func TestChangeWithoutAProposalIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("lease_action", "change"),
		str("lease_id", theLeaseID),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "proposed_lease_id is required") {
		t.Errorf("out = %v", out)
	}
}

func TestMissingLeaseActionIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "lease_action is required") {
		t.Errorf("out = %v", out)
	}
}

func TestUnknownLeaseActionIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("lease_action", "steal"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), `lease_action "steal" is not supported`) {
		t.Errorf("out = %v", out)
	}
}

func TestMissingBlobNameIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: "http://unused.invalid"},
		str("container", "my-container"),
		str("lease_action", "acquire"),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "blob_name is required") {
		t.Errorf("out = %v", out)
	}
}

// ---------------------------------------------------------------------------
// service errors
// ---------------------------------------------------------------------------

// TestConflictingLeaseIsSoftError — 409 LeaseAlreadyPresent is the ordinary
// outcome of two flows racing for the same blob, so it must land on the error
// port as data, not blow up the run.
func TestConflictingLeaseIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>LeaseAlreadyPresent</Code><Message>There is already a lease present.
RequestId:1e0b0a7c-0001
Time:2026-07-16T10:00:00.0000000Z</Message></Error>`))
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
	// The SDK surfaces the service error as a verbose block (ERROR CODE line plus
	// the raw <Error><Code>…</Code><Message>…</Message></Error> XML); the code and
	// the message must both survive into the soft error.
	if out["success"] != false ||
		!strings.Contains(msg, "LeaseAlreadyPresent") ||
		!strings.Contains(msg, "There is already a lease present.") {
		t.Errorf("out = %v, want the service's code and message", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}

// TestLeaseIDMismatchIsSoftError — 412 is what a stale lease ID earns.
func TestLeaseIDMismatchIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>LeaseIdMismatchWithLeaseOperation</Code><Message>The lease ID specified did not match the lease ID for the blob.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("lease_action", "renew"),
		str("lease_id", theLeaseID),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "LeaseIdMismatchWithLeaseOperation") {
		t.Errorf("out = %v", out)
	}
}
