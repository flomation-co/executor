package azuredevops_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

func auth(t *testing.T, orgURL string) azuredevops.Auth {
	t.Helper()
	a, err := azuredevops.GetAuth([]*core.Connection{
		{Name: "organisation_url", Type: core.ConnectionTypeString, Value: orgURL},
		{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Value: "test-pat"},
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	return a
}

// TestGetAuthDefaultsAPIVersion pins the version an unset api_version input
// resolves to. Every request carries it, so a blank default would silently
// produce ?api-version= and an unversioned response shape.
func TestGetAuthDefaultsAPIVersion(t *testing.T) {
	a := auth(t, "https://dev.azure.com/contoso")
	if a.APIVersion != azuredevops.DefaultAPIVersion {
		t.Errorf("APIVersion = %q, want %q", a.APIVersion, azuredevops.DefaultAPIVersion)
	}
}

// TestNormaliseOrgURL covers both URL shapes Azure DevOps serves and, above
// all, the vsrm derivation: a release call sent to the core host 404s, and the
// two hosts differ in SHAPE as well as name between the modern and legacy
// forms (vsrm.dev.azure.com/{org} vs {org}.vsrm.visualstudio.com).
func TestNormaliseOrgURL(t *testing.T) {
	cases := []struct {
		in          string
		core        string
		release     string
		wantErr     bool
		description string
	}{
		{in: "https://dev.azure.com/contoso", core: "https://dev.azure.com/contoso", release: "https://vsrm.dev.azure.com/contoso"},
		{in: "  https://dev.azure.com/contoso/  ", core: "https://dev.azure.com/contoso", release: "https://vsrm.dev.azure.com/contoso",
			description: "trailing slash and surrounding whitespace are survivable pastes"},
		{in: "dev.azure.com/contoso", core: "https://dev.azure.com/contoso", release: "https://vsrm.dev.azure.com/contoso",
			description: "a bare host gets https"},
		{in: "https://DEV.AZURE.COM/Contoso", core: "https://dev.azure.com/Contoso", release: "https://vsrm.dev.azure.com/Contoso",
			description: "host lower-cases, path does NOT — project and org names are case-sensitive"},
		{in: "https://contoso.visualstudio.com", core: "https://contoso.visualstudio.com", release: "https://contoso.vsrm.visualstudio.com",
			description: "the legacy form is still live and its vsrm host is a different shape"},
		{in: "https://dev.azure.com/contoso?foo=bar#frag", core: "https://dev.azure.com/contoso", release: "https://vsrm.dev.azure.com/contoso",
			description: "query and fragment are dropped so they cannot ride along on every request"},
		{in: "https://user:pass@dev.azure.com/contoso", core: "https://dev.azure.com/contoso", release: "https://vsrm.dev.azure.com/contoso",
			description: "credentials smuggled into the URL are dropped"},
		{in: "", wantErr: true},
		{in: "ftp://dev.azure.com/contoso", wantErr: true},
		{in: "https://dev.azure.com", wantErr: true, description: "no organisation in the path"},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			a, err := azuredevops.GetAuth([]*core.Connection{
				{Name: "organisation_url", Type: core.ConnectionTypeString, Value: tc.in},
				{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Value: "test-pat"},
			})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q (%s)", tc.in, tc.description)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if a.CoreBase != tc.core {
				t.Errorf("CoreBase = %q, want %q (%s)", a.CoreBase, tc.core, tc.description)
			}
			if a.ReleaseBase != tc.release {
				t.Errorf("ReleaseBase = %q, want %q (%s)", a.ReleaseBase, tc.release, tc.description)
			}
		})
	}
}

// TestGetAuthRequiresPAT pins that a missing token is a hard error rather than
// an anonymous request that 203s later with a mystifying message.
func TestGetAuthRequiresPAT(t *testing.T) {
	_, err := azuredevops.GetAuth([]*core.Connection{
		{Name: "organisation_url", Type: core.ConnectionTypeString, Value: "https://dev.azure.com/contoso"},
	})
	if err == nil || !strings.Contains(err.Error(), "Personal Access Token") {
		t.Errorf("err = %v, want a Personal Access Token requirement", err)
	}
}

// TestExecuteSendsBasicAuthWithEmptyUsername is the auth contract in one
// assertion. base64 of the BARE token is rejected by Azure DevOps — the leading
// colon (an empty username) is mandatory, and it is the single easiest thing to
// get wrong here.
func TestDoSendsBasicAuthWithEmptyUsername(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	a := auth(t, srv.URL+"/myorg")
	if _, err := azuredevops.Do(nil, a, azuredevops.Request{Method: http.MethodGet, Path: "/_apis/projects"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(":test-pat"))
	if got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
	user, pass, ok := (&http.Request{Header: http.Header{"Authorization": []string{got}}}).BasicAuth()
	if !ok || user != "" || pass != "test-pat" {
		t.Errorf("decoded as user=%q pass=%q ok=%v, want an empty username and the PAT as the password", user, pass, ok)
	}
}

// TestExecuteAlwaysSendsAPIVersion pins the rule the whole request builder
// exists to enforce: the parameter is mandatory on EVERY call, an omission does
// not fail cleanly, and no call site should have to remember it.
func TestDoAlwaysSendsAPIVersion(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	a := auth(t, srv.URL+"/myorg")

	if _, err := azuredevops.Do(nil, a, azuredevops.Request{
		Method: http.MethodGet,
		Path:   "/_apis/projects",
		Query:  url.Values{"stateFilter": {"all"}},
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Get("api-version") != "7.1" {
		t.Errorf("api-version = %q, want 7.1", got.Get("api-version"))
	}
	if got.Get("stateFilter") != "all" {
		t.Errorf("the caller's query was lost: %v", got)
	}

	// A per-request override is how the preview-only endpoints reach the wire.
	if _, err := azuredevops.Do(nil, a, azuredevops.Request{
		Method:     http.MethodGet,
		Path:       "/_apis/wit/workItems/1/comments",
		APIVersion: azuredevops.CommentsAPIVersion,
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Get("api-version") != azuredevops.CommentsAPIVersion {
		t.Errorf("api-version = %q, want the preview override %q", got.Get("api-version"), azuredevops.CommentsAPIVersion)
	}
}

// TestExecuteRoutesReleasesToVsrm pins the host split. Nothing else catches a
// release action pointed at the core host — it just 404s in production.
func TestDoRoutesReleasesToVsrm(t *testing.T) {
	a := auth(t, "https://dev.azure.com/contoso")
	if a.ReleaseBase == a.CoreBase {
		t.Fatal("ReleaseBase must differ from CoreBase for a dev.azure.com organisation")
	}
	if !strings.HasPrefix(a.ReleaseBase, "https://vsrm.dev.azure.com/") {
		t.Errorf("ReleaseBase = %q, want the vsrm host", a.ReleaseBase)
	}
}

// TestCheckResponseTreats203AsAuthFailure guards the sharpest quirk in the
// whole API. A bad or expired PAT answers 203 with an HTML sign-in page, not
// 401 — so a plain 2xx check reports SUCCESS on the most common credential
// failure there is and then explodes downstream on an HTML-to-JSON decode.
func TestCheckResponseTreats203AsAuthFailure(t *testing.T) {
	resp := &azuredevops.Response{
		StatusCode:  http.StatusNonAuthoritativeInfo,
		ContentType: "text/html; charset=utf-8",
		Body:        []byte("<!DOCTYPE html><html><body>Sign in to your account</body></html>"),
	}
	err := azuredevops.CheckResponse(resp)
	if err == nil {
		t.Fatal("203 with a sign-in page must be an error, not a success")
	}
	if !strings.Contains(err.Error(), "Personal Access Token") {
		t.Errorf("err = %v, want the message to name the Personal Access Token", err)
	}
}

// TestCheckResponseTreatsHTMLBodyAsAuthFailure covers the same redirect
// arriving with a 200. Keying on the status alone would miss it.
func TestCheckResponseTreatsHTMLBodyAsAuthFailure(t *testing.T) {
	resp := &azuredevops.Response{
		StatusCode: http.StatusOK,
		Body:       []byte("<html><head><title>Azure DevOps Services | Sign In</title></head></html>"),
	}
	if err := azuredevops.CheckResponse(resp); err == nil {
		t.Fatal("an HTML sign-in page must be an error whatever status it arrives with")
	}
}

// TestCheckResponsePassesPlainTextBody is the counterweight to the two tests
// above: build logs come back as text/plain and must NOT be mistaken for a
// sign-in page.
func TestCheckResponsePassesPlainTextBody(t *testing.T) {
	resp := &azuredevops.Response{
		StatusCode:  http.StatusOK,
		ContentType: "text/plain; charset=utf-8",
		Body:        []byte("2026-07-17T09:00:00Z Starting: Build\n"),
	}
	if err := azuredevops.CheckResponse(resp); err != nil {
		t.Errorf("a plain-text log body must pass: %v", err)
	}
}

// TestCheckResponseSurfacesErrorEnvelope pins that the operator sees Azure
// DevOps' own message rather than a bare status code.
func TestCheckResponseSurfacesErrorEnvelope(t *testing.T) {
	body, _ := json.Marshal(map[string]interface{}{
		"message": "TF401019: The Git repository with name or identifier nope does not exist.",
		"typeKey": "GitRepositoryNotFoundException",
	})
	err := azuredevops.CheckResponse(&azuredevops.Response{StatusCode: 404, Body: body})
	if err == nil || !strings.Contains(err.Error(), "TF401019") {
		t.Errorf("err = %v, want the service's own message", err)
	}
}

// TestCheckResponseAcceptableCodes pins the opt-in escape hatch.
func TestCheckResponseAcceptableCodes(t *testing.T) {
	resp := &azuredevops.Response{StatusCode: 409, Body: []byte(`{"message":"already exists"}`)}
	if err := azuredevops.CheckResponse(resp, 409); err != nil {
		t.Errorf("409 declared acceptable must pass: %v", err)
	}
	if err := azuredevops.CheckResponse(resp); err == nil {
		t.Error("409 not declared acceptable must fail")
	}
}

// TestRedactScrubsThePAT pins the rule that no error string may carry the
// credential. Both forms matter: a transport error can quote the request's
// headers, which carry the base64 form rather than the raw token.
func TestRedactScrubsThePAT(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // dial to a dead listener so the transport error is real

	a := auth(t, srv.URL+"/myorg")
	_, err := azuredevops.Do(nil, a, azuredevops.Request{Method: http.MethodGet, Path: "/_apis/projects"})
	if err == nil {
		t.Fatal("expected a transport error against a closed listener")
	}
	if strings.Contains(err.Error(), "test-pat") {
		t.Errorf("the PAT leaked into an error string: %v", err)
	}
	if strings.Contains(err.Error(), base64.StdEncoding.EncodeToString([]byte(":test-pat"))) {
		t.Errorf("the encoded PAT leaked into an error string: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

// TestListAllFollowsTheContinuationHeader pins that the token is read from the
// RESPONSE HEADER (it is not in the body at all) and echoed back as a query
// param, opaquely.
func TestListAllFollowsTheContinuationHeader(t *testing.T) {
	var seenTokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTokens = append(seenTokens, r.URL.Query().Get("continuationToken"))
		switch len(seenTokens) {
		case 1:
			w.Header().Set("x-ms-continuationtoken", "abc+/=123")
			_, _ = w.Write([]byte(`{"count":1,"value":[{"id":"1"}]}`))
		default:
			_, _ = w.Write([]byte(`{"count":1,"value":[{"id":"2"}]}`))
		}
	}))
	defer srv.Close()

	items, capped, err := azuredevops.ListAll(nil, auth(t, srv.URL+"/myorg"), azuredevops.Request{
		Method: http.MethodGet,
		Path:   "/_apis/projects",
	}, true)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if capped {
		t.Error("a two-page walk must not report the page cap")
	}
	if len(items) != 2 {
		t.Fatalf("got %d items across pages, want 2", len(items))
	}
	if len(seenTokens) != 2 || seenTokens[0] != "" || seenTokens[1] != "abc+/=123" {
		t.Errorf("tokens sent = %q, want the second request to echo the header verbatim (+ / = intact)", seenTokens)
	}
}

// TestListAllStopsOnAnEmptyPage is the infinite-pager guard. Some API versions
// return the continuation header on the LAST page too, so looping until the
// header is absent never terminates. The loop keys on "zero items" instead —
// this test fails, by timing out, if that is ever changed back.
func TestListAllStopsOnAnEmptyPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Always advertises another page — exactly the pathological server.
		w.Header().Set("x-ms-continuationtoken", "always-more")
		if calls == 1 {
			_, _ = w.Write([]byte(`{"count":1,"value":[{"id":"1"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"count":0,"value":[]}`))
	}))
	defer srv.Close()

	items, _, err := azuredevops.ListAll(nil, auth(t, srv.URL+"/myorg"), azuredevops.Request{
		Method: http.MethodGet,
		Path:   "/_apis/projects",
	}, true)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("got %d items, want 1", len(items))
	}
	if calls != 2 {
		t.Errorf("made %d calls, want 2 — the walk must stop on an empty page despite the header", calls)
	}
}

// TestListAllHonoursReturnAllOff pins that a single-page fetch does not walk.
func TestListAllHonoursReturnAllOff(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("x-ms-continuationtoken", "more")
		_, _ = w.Write([]byte(`{"count":1,"value":[{"id":"1"}]}`))
	}))
	defer srv.Close()

	items, _, err := azuredevops.ListAll(nil, auth(t, srv.URL+"/myorg"), azuredevops.Request{
		Method: http.MethodGet,
		Path:   "/_apis/projects",
	}, false)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if calls != 1 || len(items) != 1 {
		t.Errorf("calls=%d items=%d, want a single page", calls, len(items))
	}
}

// TestListAllCapsThePageWalk pins the safety backstop on a huge organisation.
func TestListAllCapsThePageWalk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-continuationtoken", "more")
		_, _ = w.Write([]byte(`{"count":1,"value":[{"id":"1"}]}`))
	}))
	defer srv.Close()

	items, capped, err := azuredevops.ListAll(nil, auth(t, srv.URL+"/myorg"), azuredevops.Request{
		Method: http.MethodGet,
		Path:   "/_apis/projects",
	}, true)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if !capped {
		t.Error("a server that never stops must be reported as capped, not as a complete list")
	}
	if len(items) != azuredevops.MaxAllPages {
		t.Errorf("got %d items, want %d (one per capped page)", len(items), azuredevops.MaxAllPages)
	}
}

// ---------------------------------------------------------------------------
// JSON-Patch translation — the single most important UX call in this node
// ---------------------------------------------------------------------------

func decodePatch(t *testing.T, ops []azuredevops.PatchOp) []map[string]interface{} {
	t.Helper()
	b, err := azuredevops.EncodePatch(ops)
	if err != nil {
		t.Fatalf("EncodePatch: %v", err)
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("the patch document is not valid JSON: %v", err)
	}
	return out
}

// TestFieldPatchTranslatesFriendlyNames is the heart of the work item UX: an
// operator types "title" and "priority", and the dotted reference names Azure
// DevOps insists on are supplied for them.
func TestFieldPatchTranslatesFriendlyNames(t *testing.T) {
	ops, err := azuredevops.FieldPatch(map[string]interface{}{
		"title":       "Checkout is broken",
		"assigned to": "jane@contoso.com",
		"priority":    1,
	})
	if err != nil {
		t.Fatalf("FieldPatch: %v", err)
	}
	got := decodePatch(t, ops)

	// Sorted by the operator's key: "assigned to", "priority", "title".
	want := []map[string]interface{}{
		{"op": "add", "path": "/fields/System.AssignedTo", "value": "jane@contoso.com"},
		{"op": "add", "path": "/fields/Microsoft.VSTS.Common.Priority", "value": float64(1)},
		{"op": "add", "path": "/fields/System.Title", "value": "Checkout is broken"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("patch document:\n got: %v\nwant: %v", got, want)
	}
}

// TestFieldPatchAlwaysUsesAdd pins the trap that "replace" is: it fails on a
// field with no current value, so an update that works on one work item 400s on
// the next purely because that field happened to be empty. "add" is
// set-or-replace and is always correct here.
func TestFieldPatchAlwaysUsesAdd(t *testing.T) {
	ops, err := azuredevops.FieldPatch(map[string]interface{}{"state": "Active", "System.Tags": "urgent"})
	if err != nil {
		t.Fatalf("FieldPatch: %v", err)
	}
	for _, op := range ops {
		if op.Op != "add" {
			t.Errorf("op for %s is %q — must be \"add\"; \"replace\" fails on an empty field", op.Path, op.Op)
		}
	}
}

// TestFieldPatchPassesReferenceNamesThrough pins the power-user path: a key
// containing a dot IS a reference name and must not be alias-looked-up.
func TestFieldPatchPassesReferenceNamesThrough(t *testing.T) {
	ops, err := azuredevops.FieldPatch(map[string]interface{}{"Custom.MyOrg.SprintGoal": "ship it"})
	if err != nil {
		t.Fatalf("FieldPatch: %v", err)
	}
	if ops[0].Path != "/fields/Custom.MyOrg.SprintGoal" {
		t.Errorf("path = %q, want the custom reference name verbatim", ops[0].Path)
	}
}

// TestFieldPatchNullRemoves pins how a field is cleared. Omitting a key leaves
// the field alone, so there has to be some way to say "make this empty".
func TestFieldPatchNullRemoves(t *testing.T) {
	ops, err := azuredevops.FieldPatch(map[string]interface{}{"assigned to": nil})
	if err != nil {
		t.Fatalf("FieldPatch: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "remove" || ops[0].Path != "/fields/System.AssignedTo" {
		t.Errorf("ops = %+v, want a single remove of System.AssignedTo", ops)
	}
	// "value" must be absent on a remove, not present-and-null.
	got := decodePatch(t, ops)
	if _, present := got[0]["value"]; present {
		t.Errorf("a remove op carries a value: %v", got[0])
	}
}

// TestFieldPatchKeepsZeroValues pins the bug a `json:"value,omitempty"` tag on
// PatchOp.Value caused: omitempty drops Go's zero values, so "", 0 and false
// marshalled to an operation with NO value at all, which Azure DevOps rejects
// with `400 Value cannot be null.` — naming neither the field nor the value.
//
// Every case below is an ordinary thing to ask for (empty a description, set a
// priority to 0, set a flag to false) and every one of them failed against the
// real service while passing every mock. A blank string must also stay DISTINCT
// from a null: "" empties the field, null removes it.
func TestFieldPatchKeepsZeroValues(t *testing.T) {
	ops, err := azuredevops.FieldPatch(map[string]interface{}{
		"description":   "",
		"priority":      0,
		"story points":  0.0,
		"System.MyFlag": false,
		"assigned to":   nil,
	})
	if err != nil {
		t.Fatalf("FieldPatch: %v", err)
	}
	got := decodePatch(t, ops)

	// Sorted by the operator's key: "System.MyFlag", "assigned to",
	// "description", "priority", "story points".
	want := []map[string]interface{}{
		{"op": "add", "path": "/fields/System.MyFlag", "value": false},
		{"op": "remove", "path": "/fields/System.AssignedTo"},
		{"op": "add", "path": "/fields/System.Description", "value": ""},
		{"op": "add", "path": "/fields/Microsoft.VSTS.Common.Priority", "value": float64(0)},
		{"op": "add", "path": "/fields/Microsoft.VSTS.Scheduling.StoryPoints", "value": float64(0)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("patch document:\n got: %v\nwant: %v", got, want)
	}

	// Spelled out, because the DeepEqual above would also pass if "value" were
	// present-but-null: an empty string must survive as an empty string.
	for _, op := range got {
		if op["op"] == "remove" {
			continue
		}
		if _, present := op["value"]; !present {
			t.Errorf("op %v has no value — Azure DevOps answers that with \"Value cannot be null\"", op)
		}
	}
}

// TestFieldPatchRawPointerEscapeHatch pins the relations path. Links are
// patched at /relations/- ("-" meaning append), which is not a field at all, so
// the friendly map would have no way to express it without this.
func TestFieldPatchRawPointerEscapeHatch(t *testing.T) {
	ops, err := azuredevops.FieldPatch(map[string]interface{}{
		"/relations/-": map[string]interface{}{
			"rel": "System.LinkTypes.Hierarchy-Reverse",
			"url": "https://dev.azure.com/contoso/_apis/wit/workItems/41",
		},
	})
	if err != nil {
		t.Fatalf("FieldPatch: %v", err)
	}
	if ops[0].Path != "/relations/-" {
		t.Errorf("path = %q, want the raw pointer untouched (no /fields/ prefix)", ops[0].Path)
	}
}

// TestFieldPatchRejectsUnknownFields pins the decision NOT to guess. Inventing
// "System.<Key>" from an unknown alias would produce a server-side 400 naming a
// field the operator never typed — a worse failure than being told up front.
func TestFieldPatchRejectsUnknownFields(t *testing.T) {
	_, err := azuredevops.FieldPatch(map[string]interface{}{"wibble": "x"})
	if err == nil {
		t.Fatal("an unknown field name must be rejected, not guessed at")
	}
	if !strings.Contains(err.Error(), "wibble") || !strings.Contains(err.Error(), "reference name") {
		t.Errorf("err = %v, want it to name the field and point at reference names", err)
	}
}

// TestFieldPatchRejectsAnEmptyMap pins that a no-op patch fails loudly. Azure
// DevOps answers an empty document with an unhelpful 400.
func TestFieldPatchRejectsAnEmptyMap(t *testing.T) {
	if _, err := azuredevops.FieldPatch(map[string]interface{}{}); err == nil {
		t.Fatal("an empty field map must be rejected")
	}
}

// TestFieldPatchIsDeterministic pins the sort. Go map iteration order is
// random, so without it the same inputs would produce a different document (and
// a flaky test) on every run.
func TestFieldPatchIsDeterministic(t *testing.T) {
	fields := map[string]interface{}{"title": "a", "state": "b", "priority": 1, "tags": "c", "effort": 2}
	first, err := azuredevops.FieldPatch(fields)
	if err != nil {
		t.Fatalf("FieldPatch: %v", err)
	}
	for i := 0; i < 20; i++ {
		again, err := azuredevops.FieldPatch(fields)
		if err != nil {
			t.Fatalf("FieldPatch: %v", err)
		}
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("patch order is not stable:\n%v\n%v", first, again)
		}
	}
}

// TestResolveFieldNameAliases spot-checks the alias table across both
// namespaces an operator meets.
func TestResolveFieldNameAliases(t *testing.T) {
	cases := map[string]string{
		"Title":          "System.Title",
		"  state  ":      "System.State",
		"area path":      "System.AreaPath",
		"iteration_path": "System.IterationPath",
		"story points":   "Microsoft.VSTS.Scheduling.StoryPoints",
		"repro steps":    "Microsoft.VSTS.TCM.ReproSteps",
		"System.Title":   "System.Title",
	}
	for in, want := range cases {
		got, err := azuredevops.ResolveFieldName(in)
		if err != nil {
			t.Errorf("ResolveFieldName(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ResolveFieldName(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Work item batching
// ---------------------------------------------------------------------------

// TestFetchWorkItemsChunksAtTheServerCap pins the 200-id limit. Exceeding it is
// a flat 400, so a WIQL query returning 250 references would fail entirely
// without this.
func TestFetchWorkItemsChunksAtTheServerCap(t *testing.T) {
	var chunkSizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			IDs []int `json:"ids"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		chunkSizes = append(chunkSizes, len(body.IDs))
		items := make([]string, 0, len(body.IDs))
		for _, id := range body.IDs {
			items = append(items, fmt.Sprintf(`{"id":%d}`, id))
		}
		_, _ = fmt.Fprintf(w, `{"count":%d,"value":[%s]}`, len(items), strings.Join(items, ","))
	}))
	defer srv.Close()

	ids := make([]int, 450)
	for i := range ids {
		ids[i] = i + 1
	}
	items, err := azuredevops.FetchWorkItems(nil, auth(t, srv.URL+"/myorg"), "MyProject", ids, nil, "")
	if err != nil {
		t.Fatalf("FetchWorkItems: %v", err)
	}
	if len(items) != 450 {
		t.Errorf("got %d items, want 450", len(items))
	}
	want := []int{200, 200, 50}
	if !reflect.DeepEqual(chunkSizes, want) {
		t.Errorf("chunk sizes = %v, want %v (the server caps a batch at %d)", chunkSizes, want, azuredevops.WorkItemBatchLimit)
	}
}

// TestFetchWorkItemsFieldsBeatExpand pins the mutual exclusion at the one place
// both could be sent. The service rejects the pair.
func TestFetchWorkItemsFieldsBeatExpand(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"count":0,"value":[]}`))
	}))
	defer srv.Close()

	if _, err := azuredevops.FetchWorkItems(nil, auth(t, srv.URL+"/myorg"), "", []int{1},
		[]string{"System.Title"}, "all"); err != nil {
		t.Fatalf("FetchWorkItems: %v", err)
	}
	if _, ok := body["$expand"]; ok {
		t.Errorf("both fields and $expand were sent: %v — Azure DevOps rejects the pair", body)
	}
	if _, ok := body["fields"]; !ok {
		t.Errorf("fields was dropped: %v", body)
	}
}

// ---------------------------------------------------------------------------
// Small helpers with sharp edges
// ---------------------------------------------------------------------------

// TestFullRefName pins the branch-name expansion. The Git APIs silently 400 on
// a bare "main", which is one of the more common first-run failures.
func TestFullRefName(t *testing.T) {
	cases := map[string]string{
		"main":                 "refs/heads/main",
		"feature/checkout-fix": "refs/heads/feature/checkout-fix",
		"refs/heads/main":      "refs/heads/main",
		"refs/pull/12/merge":   "refs/pull/12/merge",
		"  release/2.0  ":      "refs/heads/release/2.0",
		"":                     "",
	}
	for in, want := range cases {
		if got := azuredevops.FullRefName(in); got != want {
			t.Errorf("FullRefName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestNormalisePipelineVariables pins the friendly-to-wire translation. A bare
// scalar is what an operator will type and is a 400 unwrapped; an object that
// already carries the envelope must survive untouched so isSecret stays
// reachable.
func TestNormalisePipelineVariables(t *testing.T) {
	got := azuredevops.NormalisePipelineVariables(map[string]interface{}{
		"releaseTag": "v1.2.3",
		"retries":    3,
		"apiKey":     map[string]interface{}{"value": "abc", "isSecret": true},
	})
	want := map[string]interface{}{
		"releaseTag": map[string]interface{}{"value": "v1.2.3"},
		"retries":    map[string]interface{}{"value": 3},
		"apiKey":     map[string]interface{}{"value": "abc", "isSecret": true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if azuredevops.NormalisePipelineVariables(nil) != nil {
		t.Error("a nil map must stay nil so the key is omitted from the body entirely")
	}
}

// TestIDOfStringifiesJSONNumbers pins the float64 case: every numeric id
// arrives as a float64 after a JSON decode, and 42 must read as "42", never
// "4.2e+01".
func TestIDOfStringifiesJSONNumbers(t *testing.T) {
	cases := []struct {
		obj  map[string]interface{}
		want string
	}{
		{map[string]interface{}{"id": float64(42)}, "42"},
		{map[string]interface{}{"id": float64(1234567890123)}, "1234567890123"},
		{map[string]interface{}{"id": "6dc1b0f3-6a9a-4b1e-9c1d-000000000000"}, "6dc1b0f3-6a9a-4b1e-9c1d-000000000000"},
		{map[string]interface{}{"id": 7}, "7"},
		{map[string]interface{}{}, ""},
		{map[string]interface{}{"id": nil}, ""},
	}
	for _, tc := range cases {
		if got := azuredevops.IDOf(tc.obj); got != tc.want {
			t.Errorf("IDOf(%v) = %q, want %q", tc.obj, got, tc.want)
		}
	}
}

// TestParseIDList pins that a bad entry is named rather than silently dropped.
func TestParseIDList(t *testing.T) {
	ids, err := azuredevops.ParseIDList(" 42, 43 ,44 ", "Work Item IDs")
	if err != nil {
		t.Fatalf("ParseIDList: %v", err)
	}
	if !reflect.DeepEqual(ids, []int{42, 43, 44}) {
		t.Errorf("ids = %v, want [42 43 44]", ids)
	}
	if _, err := azuredevops.ParseIDList("42,bug-1", "Work Item IDs"); err == nil || !strings.Contains(err.Error(), "bug-1") {
		t.Errorf("err = %v, want the offending entry named", err)
	}
	if _, err := azuredevops.ParseIDList("", "Work Item IDs"); err == nil {
		t.Error("an empty list must be rejected")
	}
}

// TestClampLimit pins the bounds.
func TestClampLimit(t *testing.T) {
	cases := []struct {
		limit int
		set   bool
		want  int
	}{
		{0, false, azuredevops.DefaultPageLimit},
		{0, true, azuredevops.DefaultPageLimit},
		{-5, true, azuredevops.DefaultPageLimit},
		{10, true, 10},
		{99999, true, azuredevops.MaxPageLimit},
	}
	for _, tc := range cases {
		if got := azuredevops.ClampLimit(tc.limit, tc.set); got != tc.want {
			t.Errorf("ClampLimit(%d, %v) = %d, want %d", tc.limit, tc.set, got, tc.want)
		}
	}
}

// TestDecodeListHandlesAnAbsentValue pins that a body with no "value" key reads
// as an empty list rather than nil — a nil results output serialises as null
// and breaks a downstream Loop.
func TestDecodeListHandlesAnAbsentValue(t *testing.T) {
	items, err := azuredevops.DecodeList(&azuredevops.Response{Body: []byte(`{"count":0}`)})
	if err != nil {
		t.Fatalf("DecodeList: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Errorf("items = %v, want a non-nil empty slice", items)
	}
}

// TestErrorResultShape pins the soft-failure contract the engine reads.
func TestErrorResultShape(t *testing.T) {
	out := azuredevops.ErrorResult("boom")
	if out["success"] != false || out["error"] != "boom" || out["tool_result"] != "boom" {
		t.Errorf("ErrorResult = %v", out)
	}
}
