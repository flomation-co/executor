package azure_tables_service_get_properties

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// devKey is Azurite's well-known development account key — published in
// Microsoft's own documentation, not a secret.
const devKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

// baseInputs is the Azurite-shaped credential block: the account in the URL
// path rather than the host, which is the endpoint style these tests and the
// emulator share.
func baseInputs(endpoint string, extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: devKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint + "/devstoreaccount1"},
	}
	return append(inputs, extra...)
}

func str(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: v}
}

func obj(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: v}
}

func flag(name string, v bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: v}
}

// errorServer answers every request with one Table service error. The
// x-ms-error-code header is what azcore reads the code from, exactly as the
// real service and Azurite send it.
func errorServer(status int, code string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-error-code", code)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"odata.error":{"code":"` + code + `","message":{"value":"the service said no"}}}`))
	}))
}

func mustSoftFail(t *testing.T, out map[string]interface{}, err error, want string) {
	t.Helper()
	if err != nil {
		t.Fatalf("must be a soft failure, got hard error: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("expected failure, got %v", out)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, want) {
		t.Errorf("error = %q, want it to mention %q", msg, want)
	}
}

func mustSucceed(t *testing.T, out map[string]interface{}, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("expected success, got error %v", out["error"])
	}
}

const propsXML = `<?xml version="1.0" encoding="utf-8"?>
<StorageServiceProperties>
  <Logging>
    <Version>1.0</Version>
    <Delete>true</Delete>
    <Read>false</Read>
    <Write>true</Write>
    <RetentionPolicy><Enabled>true</Enabled><Days>7</Days></RetentionPolicy>
  </Logging>
  <HourMetrics>
    <Version>1.0</Version><Enabled>true</Enabled><IncludeAPIs>true</IncludeAPIs>
    <RetentionPolicy><Enabled>false</Enabled></RetentionPolicy>
  </HourMetrics>
  <MinuteMetrics>
    <Version>1.0</Version><Enabled>false</Enabled>
    <RetentionPolicy><Enabled>false</Enabled></RetentionPolicy>
  </MinuteMetrics>
  <Cors>
    <CorsRule>
      <AllowedOrigins>https://example.com</AllowedOrigins>
      <AllowedMethods>GET</AllowedMethods>
      <AllowedHeaders>x-ms-meta-*</AllowedHeaders>
      <ExposedHeaders>x-ms-meta-*</ExposedHeaders>
      <MaxAgeInSeconds>300</MaxAgeInSeconds>
    </CorsRule>
  </Cors>
</StorageServiceProperties>`

func TestExecuteReadsServiceProperties(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(propsXML))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL))
	mustSucceed(t, out, err)

	if !strings.Contains(gotQuery, "restype=service") || !strings.Contains(gotQuery, "comp=properties") {
		t.Errorf("query = %s", gotQuery)
	}
	if out["id"] != "devstoreaccount1" {
		t.Errorf("id = %v", out["id"])
	}
	result := out["result"].(map[string]interface{})
	logging := result["logging"].(map[string]interface{})
	if logging["delete"] != true || logging["read"] != false {
		t.Errorf("logging = %v", logging)
	}
	cors := result["cors"].([]interface{})
	if len(cors) != 1 || cors[0].(map[string]interface{})["max_age_in_seconds"] != 300 {
		t.Errorf("cors = %v", cors)
	}
	if result["geo_replication"] != nil {
		t.Errorf("geo-replication must be opt-in, not fetched by default: %v", result)
	}
}

func TestExecuteGeoReplicationIsOptIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		if strings.Contains(r.URL.RawQuery, "comp=stats") {
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<StorageServiceStats><GeoReplication><Status>live</Status><LastSyncTime>Fri, 17 Jul 2026 09:00:00 GMT</LastSyncTime></GeoReplication></StorageServiceStats>`))
			return
		}
		_, _ = w.Write([]byte(propsXML))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL, flag("include_geo_replication", true)))
	mustSucceed(t, out, err)

	geo := out["result"].(map[string]interface{})["geo_replication"].(map[string]interface{})
	if geo["status"] != "live" {
		t.Errorf("geo = %v", geo)
	}
}

// Asking a PRIMARY endpoint for geo statistics always fails. Swallowing that
// would leave an absent field reading as "not replicated", which is a
// different and wrong answer.
func TestExecuteGeoReplicationFailureIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "comp=stats") {
			w.Header().Set("x-ms-error-code", "InsufficientAccountPermissions")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(propsXML))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL, flag("include_geo_replication", true)))
	mustSoftFail(t, out, err, "-secondary endpoint")
}

func TestExecuteForbiddenIsSoftError(t *testing.T) {
	srv := errorServer(http.StatusForbidden, "AuthenticationFailed")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL))
	mustSoftFail(t, out, err, "account name or key was rejected")
}
