package files_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"

	share_create "flomation.app/automate/executor/actions/azure/files/share_create"
	share_delete "flomation.app/automate/executor/actions/azure/files/share_delete"
	share_get_all "flomation.app/automate/executor/actions/azure/files/share_get_all"
	share_get_properties "flomation.app/automate/executor/actions/azure/files/share_get_properties"
	share_get_stats "flomation.app/automate/executor/actions/azure/files/share_get_stats"
	share_set_metadata "flomation.app/automate/executor/actions/azure/files/share_set_metadata"
	share_set_properties "flomation.app/automate/executor/actions/azure/files/share_set_properties"
)

// ---------------------------------------------------------------------------
// share_create
// ---------------------------------------------------------------------------

func TestShareCreate(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotQuota, gotTier, gotMeta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		gotQuota = r.Header.Get("x-ms-share-quota")
		gotTier = r.Header.Get("x-ms-access-tier")
		gotMeta = r.Header.Get("x-ms-meta-team")
		w.Header().Set("ETag", `"0x8D"`)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := share_create.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"),
		integer("quota", 100),
		str("access_tier", "Hot"),
		obj("metadata", `{"team":"ops"}`),
	))
	wantSuccess(t, out, err)

	if gotMethod != http.MethodPut || gotPath != "/my-share" || gotQuery != "restype=share" {
		t.Errorf("request = %s %s?%s, want PUT /my-share?restype=share", gotMethod, gotPath, gotQuery)
	}
	if gotQuota != "100" || gotTier != "Hot" || gotMeta != "ops" {
		t.Errorf("headers: quota=%q tier=%q meta=%q", gotQuota, gotTier, gotMeta)
	}
	if out["id"] != "my-share" {
		t.Errorf("id = %v, want the share name", out["id"])
	}
}

func TestShareCreateErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusConflict, "ShareAlreadyExists", "The specified share already exists.")
	}))
	defer srv.Close()

	// An already-existing share is named rather than passed through as a 409.
	out, err := share_create.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "my-share")))
	wantSoftFailure(t, out, err, `share "my-share" already exists`)
	wantNoSecretLeak(t, out)

	// The name rule is enforced CLIENT-side so the operator gets the rule back,
	// not a signed request that 400s.
	out, err = share_create.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "My_Share")))
	wantSoftFailure(t, out, err, "3-63 lowercase letters")

	// The quota bounds likewise: the service answers an out-of-range quota with
	// a bare 400 that names the header, not the field.
	out, err = share_create.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "my-share"), integer("quota", 999999)))
	wantSoftFailure(t, out, err, "quota must be between 1 and 102400")
}

// ---------------------------------------------------------------------------
// share_get_all
// ---------------------------------------------------------------------------

func TestShareGetAll(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		xmlBody(w, `<EnumerationResults>
			<Shares>
				<Share><Name>alpha</Name><Properties><Quota>100</Quota></Properties><Metadata><team>ops</team></Metadata></Share>
				<Share><Name>beta</Name><Properties><Quota>5120</Quota></Properties></Share>
			</Shares>
			<NextMarker />
		</EnumerationResults>`)
	}))
	defer srv.Close()

	out, err := share_get_all.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("prefix", "a"),
		str("include", "metadata,snapshots"),
		integer("limit", 10),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["count"] != 2 {
		t.Fatalf("count = %v, want 2 (out: %v)", out["count"], out)
	}
	results := out["results"].([]interface{})
	first := results[0].(map[string]interface{})
	if first["name"] != "alpha" {
		t.Errorf("first share = %#v", first)
	}
	if first["metadata"].(map[string]interface{})["team"] != "ops" {
		t.Errorf("metadata = %#v", first["metadata"])
	}
	for _, want := range []string{"comp=list", "prefix=a", "include=metadata%2Csnapshots", "maxresults=10"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestShareGetAllRejectsABlobIncludeToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("an unknown include token must be caught before the request is sent")
	}))
	defer srv.Close()

	// "tags" is a BLOB token. The service answers one with a flat 400 that names
	// nothing, so it is caught here where the message can list what works.
	out, err := share_get_all.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("include", "tags")))
	wantSoftFailure(t, out, err, "is not supported")
}

// ---------------------------------------------------------------------------
// share_delete
// ---------------------------------------------------------------------------

// TestShareDeleteIncludesSnapshotsByDefault — without the header a share that
// has snapshots is refused outright, and "delete the share" almost never means
// "unless somebody snapshotted it, in which case do nothing".
func TestShareDeleteIncludesSnapshotsByDefault(t *testing.T) {
	var gotMethod, gotSnapshots string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotSnapshots = r.Header.Get("x-ms-delete-snapshots")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := share_delete.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "my-share")))
	result := wantSuccess(t, out, err)
	if gotMethod != http.MethodDelete || gotSnapshots != "include" {
		t.Errorf("request = %s with x-ms-delete-snapshots %q, want DELETE with include", gotMethod, gotSnapshots)
	}
	if result["deleted"] != true {
		t.Errorf("result = %#v", result)
	}

	// "none" is the explicit opt-out: no header, so the service's own refusal
	// stands.
	out, err = share_delete.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "my-share"), str("delete_snapshots", "none")))
	wantSuccess(t, out, err)
	if gotSnapshots != "" {
		t.Errorf("delete_snapshots=none sent %q, want no header", gotSnapshots)
	}
}

func TestShareDeleteErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusNotFound, "ShareNotFound", "The specified share does not exist.")
	}))
	defer srv.Close()

	out, err := share_delete.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "gone")))
	wantSoftFailure(t, out, err, "ShareNotFound")
	if strings.Contains(out["error"].(string), "RequestId") {
		t.Errorf("RequestId noise must be trimmed: %v", out["error"])
	}

	out, err = share_delete.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "s"), str("delete_snapshots", "maybe")))
	wantSoftFailure(t, out, err, "is not valid")
}

// ---------------------------------------------------------------------------
// share_get_properties
// ---------------------------------------------------------------------------

func TestShareGetProperties(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "restype=share" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("x-ms-share-quota", "5120")
		w.Header().Set("x-ms-access-tier", "Hot")
		w.Header().Set("x-ms-meta-team", "ops")
		w.Header().Set("ETag", `"0x8D"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := share_get_properties.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "my-share")))
	result := wantSuccess(t, out, err)

	// A share's properties arrive in HEADERS — there is no body to parse.
	props := result["properties"].(map[string]interface{})
	if props["shareQuota"] != int64(5120) || props["accessTier"] != "Hot" {
		t.Errorf("properties = %#v", props)
	}
	if result["metadata"].(map[string]interface{})["team"] != "ops" {
		t.Errorf("metadata = %#v", result["metadata"])
	}
}

func TestShareGetPropertiesNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusNotFound, "ShareNotFound", "The specified share does not exist.")
	}))
	defer srv.Close()

	out, err := share_get_properties.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "gone")))
	wantSoftFailure(t, out, err, "ShareNotFound")
}

// ---------------------------------------------------------------------------
// share_get_stats
// ---------------------------------------------------------------------------

func TestShareGetStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "comp=stats") {
			t.Errorf("query = %q, want comp=stats", r.URL.RawQuery)
		}
		xmlBody(w, `<ShareStats><ShareUsageBytes>1234567</ShareUsageBytes></ShareStats>`)
	}))
	defer srv.Close()

	out, err := share_get_stats.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "my-share")))
	result := wantSuccess(t, out, err)
	if out["usage_bytes"] != int64(1234567) {
		t.Errorf("usage_bytes = %#v, want int64(1234567)", out["usage_bytes"])
	}
	if result["shareUsageBytes"] != int64(1234567) {
		t.Errorf("result = %#v", result)
	}
	if !strings.Contains(out["tool_result"].(string), "1234567 bytes") {
		t.Errorf("tool_result = %q", out["tool_result"])
	}
}

func TestShareGetStatsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		azureError(w, http.StatusNotFound, "ShareNotFound", "The specified share does not exist.")
	}))
	defer srv.Close()

	out, err := share_get_stats.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "gone")))
	wantSoftFailure(t, out, err, "ShareNotFound")
}

// ---------------------------------------------------------------------------
// share_set_properties
// ---------------------------------------------------------------------------

func TestShareSetProperties(t *testing.T) {
	var gotQuota, gotTier, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuota, gotTier, gotQuery = r.Header.Get("x-ms-share-quota"), r.Header.Get("x-ms-access-tier"), r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := share_set_properties.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), integer("quota", 200), str("access_tier", "Cool")))
	wantSuccess(t, out, err)
	if gotQuota != "200" || gotTier != "Cool" {
		t.Errorf("headers: quota=%q tier=%q", gotQuota, gotTier)
	}
	for _, want := range []string{"restype=share", "comp=properties"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

// TestShareSetPropertiesRefusesANoOp — an omitted header means "leave it
// alone", so a call with neither field set is a no-op the service happily
// reports as a success. That is worse than an error: the flow says it changed
// the quota and it did not.
func TestShareSetPropertiesRefusesANoOp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a no-op must not reach the service")
	}))
	defer srv.Close()

	out, err := share_set_properties.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "my-share")))
	wantSoftFailure(t, out, err, "at least one of quota or access_tier")
}

// ---------------------------------------------------------------------------
// share_set_metadata
// ---------------------------------------------------------------------------

func TestShareSetMetadata(t *testing.T) {
	got := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range r.Header {
			if strings.HasPrefix(strings.ToLower(k), "x-ms-meta-") {
				got[strings.ToLower(k)] = v[0]
			}
		}
		if !strings.Contains(r.URL.RawQuery, "comp=metadata") {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := share_set_metadata.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), obj("metadata", `{"team":"ops","env":"prod"}`)))
	wantSuccess(t, out, err)
	if got["x-ms-meta-team"] != "ops" || got["x-ms-meta-env"] != "prod" {
		t.Errorf("metadata headers = %#v", got)
	}
}

func TestShareSetMetadataErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// A missing object is not "clear the metadata" — {} is. Absent is a
	// misconfiguration, and clearing a share's metadata by accident is not a
	// recoverable mistake.
	out, err := share_set_metadata.Execute(&core.Flow{}, nil, authInputs(srv.URL, str("share", "my-share")))
	wantSoftFailure(t, out, err, "metadata is required")

	// Metadata names travel as x-ms-meta-{name} headers and must be valid C#
	// identifiers; the service enforces it with an opaque error.
	out, err = share_set_metadata.Execute(&core.Flow{}, nil, authInputs(srv.URL,
		str("share", "my-share"), obj("metadata", `{"1bad":"x"}`)))
	wantSoftFailure(t, out, err, "is invalid")
}
