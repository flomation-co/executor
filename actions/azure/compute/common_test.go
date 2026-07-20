package compute

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	core "flomation.app/automate/executor"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// validCreds is the smallest input set GetAuth accepts. The values are
// well-formed but fictional — azidentity builds the credential offline (no
// token is requested until an actual ARM call), so GetAuth succeeds without a
// network round-trip.
func validCreds() []*core.Connection {
	return []*core.Connection{
		{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Value: "00000000-0000-0000-0000-000000000000"},
		{Name: "azure_client_id", Type: core.ConnectionTypeString, Value: "11111111-1111-1111-1111-111111111111"},
		{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Value: "s3cr3t-value"},
		{Name: "subscription_id", Type: core.ConnectionTypeString, Value: "22222222-2222-2222-2222-222222222222"},
		{Name: "resource_group", Type: core.ConnectionTypeString, Value: "my-rg"},
	}
}

func TestGetAuth(t *testing.T) {
	a, err := GetAuth(validCreds())
	if err != nil {
		t.Fatalf("GetAuth(valid) = %v, want nil", err)
	}
	if a.cred == nil {
		t.Error("GetAuth did not build a credential")
	}
	if a.SubscriptionID == "" || a.ResourceGroup != "my-rg" {
		t.Errorf("GetAuth did not carry scope: sub=%q rg=%q", a.SubscriptionID, a.ResourceGroup)
	}
	if _, err := a.VMClient(); err != nil {
		t.Errorf("VMClient() = %v, want nil", err)
	}

	// Each required field, when missing, must produce an error that names it.
	for field, want := range map[string]string{
		"azure_tenant_id":     "tenant",
		"azure_client_id":     "client",
		"azure_client_secret": "secret",
		"subscription_id":     "subscription",
	} {
		inputs := validCreds()
		for _, c := range inputs {
			if c.Name == field {
				c.Value = ""
			}
		}
		_, err := GetAuth(inputs)
		if err == nil {
			t.Errorf("GetAuth(missing %s) = nil, want an error", field)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("GetAuth(missing %s) error = %q, want it to mention %q", field, err, want)
		}
	}

	// A tenant with URL-unsafe characters is rejected before azidentity sees it.
	inputs := validCreds()
	for _, c := range inputs {
		if c.Name == "azure_tenant_id" {
			c.Value = "bad/tenant"
		}
	}
	if _, err := GetAuth(inputs); err == nil {
		t.Error("GetAuth(tenant with '/') = nil, want an error")
	}
}

func TestTagMap(t *testing.T) {
	in := func(v string) []*core.Connection {
		return []*core.Connection{{Name: "tags", Type: core.ConnectionTypeString, Value: v}}
	}
	// blank → nil, no error
	if m, err := TagMap("tags", in("")); err != nil || m != nil {
		t.Errorf("TagMap(blank) = (%v, %v), want (nil, nil)", m, err)
	}
	// valid object
	m, err := TagMap("tags", in(`{"env":"prod","owner":"ops"}`))
	if err != nil {
		t.Fatalf("TagMap(valid) = %v", err)
	}
	if len(m) != 2 || m["env"] == nil || *m["env"] != "prod" {
		t.Errorf("TagMap(valid) = %v, want env=prod owner=ops", m)
	}
	// malformed JSON → error
	if _, err := TagMap("tags", in(`{"env":`)); err == nil {
		t.Error("TagMap(malformed) = nil error, want an error")
	}
	// well-formed JSON that isn't an object of strings → error (ARM tags are
	// string→string). Covers Dan's "double-check the contract" note.
	for _, bad := range []string{`{"a":1}`, `[1,2]`, `"just a string"`, `42`} {
		if _, err := TagMap("tags", in(bad)); err == nil {
			t.Errorf("TagMap(%q) = nil error, want an error", bad)
		}
	}
	// JSON null decodes to an empty map with no error; resource_tag_set's own
	// len==0 guard is what rejects "no tags", so this must stay non-erroring.
	if m, err := TagMap("tags", in(`null`)); err != nil || len(m) != 0 {
		t.Errorf("TagMap(null) = (%v, %v), want (empty, nil)", m, err)
	}
}

func TestRequiredInt(t *testing.T) {
	in := func(v string) []*core.Connection {
		return []*core.Connection{{Name: "priority", Type: core.ConnectionTypeInteger, Value: v}}
	}
	if n, err := RequiredInt("priority", in("100")); err != nil || n != 100 {
		t.Errorf("RequiredInt(100) = (%d, %v), want (100, nil)", n, err)
	}
	if _, err := RequiredInt("priority", in("")); err == nil {
		t.Error("RequiredInt(blank) = nil, want an error")
	}
	if _, err := RequiredInt("priority", in("abc")); err == nil {
		t.Error("RequiredInt(non-number) = nil, want an error")
	}
}

func TestInputStrings(t *testing.T) {
	in := []*core.Connection{{Name: "ids", Type: core.ConnectionTypeString, Value: " a , ,b,c "}}
	got := InputStrings("ids", in)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("InputStrings = %v, want [a b c]", got)
	}
	if InputStrings("ids", []*core.Connection{{Name: "ids", Value: ""}}) != nil {
		t.Error("InputStrings(blank) should be nil")
	}
}

func TestErrorResult(t *testing.T) {
	r := ErrorResult("boom")
	if r["success"] != false || r["error"] != "boom" || r["tool_result"] != "boom" {
		t.Errorf("ErrorResult shape = %v", r)
	}
}

func TestAzureError(t *testing.T) {
	if AzureError(nil) != "" {
		t.Error("AzureError(nil) should be empty")
	}
	// A ResponseError is summarised by its code, not dumped whole.
	re := &azcore.ResponseError{ErrorCode: "ResourceNotFound", StatusCode: http.StatusNotFound}
	got := AzureError(re)
	if !strings.Contains(got, "ResourceNotFound") {
		t.Errorf("AzureError = %q, want it to name the code", got)
	}

	// The Auth-bound variant redacts the client secret.
	a := Auth{ClientSecret: "s3cr3t-value"}
	if strings.Contains(a.AzureError(errors.New("failed with s3cr3t-value in it")), "s3cr3t-value") {
		t.Error("Auth.AzureError leaked the client secret")
	}
}

func TestStr(t *testing.T) {
	if Str(nil) != "" {
		t.Error("Str(nil) should be empty")
	}
	v := "x"
	if Str(&v) != "x" {
		t.Error("Str(&v) should deref")
	}
}
