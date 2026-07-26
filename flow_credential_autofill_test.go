package core

import "testing"

// The auto-fill exists because several providers capture a value at OAuth time
// that the operator then retypes by hand — Salesforce's instance_url, QuickBooks'
// realm_id, Xero's tenant_id. Retyping is a trap, not just a chore: the
// Salesforce field's placeholder invites pasting from the browser address bar,
// which is the LIGHTNING host, not the API one, and fails looking like a broken
// integration.
//
// These test the rewrite in isolation. The links come from the manifest at
// runtime, so the tests seed the cache directly rather than depending on which
// actions happen to declare one.
func withLinks(t *testing.T, links map[string]map[string]string) {
	t.Helper()
	prev := credentialMetaLinks
	credentialMetaLinks = links
	t.Cleanup(func() { credentialMetaLinks = prev })
}

func inputs(pairs ...string) []*Connection {
	out := make([]*Connection, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, &Connection{Name: pairs[i], Value: pairs[i+1]})
	}
	return out
}

func valueOf(in []*Connection, name string) string {
	for _, c := range in {
		if c == nil {
			continue
		}
		if c.Name == name {
			s, _ := c.Value.(string)
			return s
		}
	}
	return "<absent>"
}

func TestBlankLinkedInputIsFilledFromTheBoundCredential(t *testing.T) {
	withLinks(t, map[string]map[string]string{
		"crm/salesforce/account_get_all": {"instance_url": "instance_url"},
	})
	in := inputs("access_token", "${credentials.salesforce-test}", "instance_url", "")

	autofillFromCredential("crm/salesforce/account_get_all", in)

	if got := valueOf(in, "instance_url"); got != "${credentials.salesforce-test.instance_url}" {
		t.Errorf("blank linked input should reference the credential's metadata, got %q", got)
	}
	// The connection itself must be left alone.
	if got := valueOf(in, "access_token"); got != "${credentials.salesforce-test}" {
		t.Errorf("the credential reference must not be rewritten, got %q", got)
	}
}

// THE case that must not regress. An operator pasting a raw token has no
// credential to read from, and the field has to keep behaving exactly as before.
func TestPastedTokenPathIsUntouched(t *testing.T) {
	withLinks(t, map[string]map[string]string{
		"crm/salesforce/account_get_all": {"instance_url": "instance_url"},
	})
	in := inputs("access_token", "${secrets.salesforce_token}", "instance_url", "")

	autofillFromCredential("crm/salesforce/account_get_all", in)

	if got := valueOf(in, "instance_url"); got != "" {
		t.Errorf("with no credential bound the input must stay blank so the existing required-field error still fires, got %q", got)
	}
}

// Anything typed wins. Auto-fill is a default, never an override — an operator
// pointing at a different host (a sandbox, a My Domain alias) must be obeyed.
func TestATypedValueIsNeverOverwritten(t *testing.T) {
	withLinks(t, map[string]map[string]string{
		"crm/salesforce/account_get_all": {"instance_url": "instance_url"},
	})
	for _, typed := range []string{
		"https://other.my.salesforce.com",
		"${credentials.another-org.instance_url}",
		"${env.SF_HOST}",
	} {
		in := inputs("access_token", "${credentials.salesforce-test}", "instance_url", typed)
		autofillFromCredential("crm/salesforce/account_get_all", in)
		if got := valueOf(in, "instance_url"); got != typed {
			t.Errorf("typed value %q was overwritten with %q", typed, got)
		}
	}
}

// Whitespace is not a value. An operator who tabbed through the field should get
// the same result as one who never touched it.
func TestWhitespaceCountsAsBlank(t *testing.T) {
	withLinks(t, map[string]map[string]string{
		"crm/salesforce/account_get_all": {"instance_url": "instance_url"},
	})
	in := inputs("access_token", "${credentials.salesforce-test}", "instance_url", "   ")
	autofillFromCredential("crm/salesforce/account_get_all", in)
	if got := valueOf(in, "instance_url"); got != "${credentials.salesforce-test.instance_url}" {
		t.Errorf("whitespace should read as blank, got %q", got)
	}
}

// A ${credentials.NAME.key} reference is already a metadata VALUE, not the
// connection. Treating it as the bound credential would produce a nonsense
// double reference like ${credentials.instance_url.instance_url}.
func TestAMetadataReferenceIsNotMistakenForTheCredential(t *testing.T) {
	withLinks(t, map[string]map[string]string{
		"crm/salesforce/account_get_all": {"instance_url": "instance_url"},
	})
	in := inputs("some_other_field", "${credentials.salesforce-test.realm_id}", "instance_url", "")
	autofillFromCredential("crm/salesforce/account_get_all", in)
	if got := valueOf(in, "instance_url"); got != "" {
		t.Errorf("a metadata reference is not a credential binding, got %q", got)
	}
}

// An action declaring no links must be untouched — this runs for every node in
// every flow, so it has to be inert by default.
func TestActionsWithoutLinksAreUntouched(t *testing.T) {
	withLinks(t, map[string]map[string]string{
		"crm/salesforce/account_get_all": {"instance_url": "instance_url"},
	})
	in := inputs("access_token", "${credentials.salesforce-test}", "instance_url", "")
	autofillFromCredential("some/other/action", in)
	if got := valueOf(in, "instance_url"); got != "" {
		t.Errorf("an unlinked action must not be modified, got %q", got)
	}
}

func TestNilInputsAreToleratedRatherThanPanicking(t *testing.T) {
	withLinks(t, map[string]map[string]string{
		"crm/salesforce/account_get_all": {"instance_url": "instance_url"},
	})
	in := []*Connection{nil, {Name: "access_token", Value: "${credentials.c}"}, nil, {Name: "instance_url", Value: ""}}
	autofillFromCredential("crm/salesforce/account_get_all", in)
	if got := valueOf(in, "instance_url"); got != "${credentials.c.instance_url}" {
		t.Errorf("nil entries should be skipped, not fatal, got %q", got)
	}
}

func TestCredentialRefPatternRejectsNearMisses(t *testing.T) {
	for _, bad := range []string{
		"${credentials.a.b}",           // a metadata value, not the credential
		"prefix ${credentials.a}",      // embedded, not a whole-value binding
		"${credentials.a} suffix",      //
		"${secrets.a}",                 //
		"${credentials.}",              //
		"${credentials.bad.name.here}", //
	} {
		if credentialRefPattern.MatchString(bad) {
			t.Errorf("%q must not be read as a credential binding", bad)
		}
	}
	for _, good := range []string{"${credentials.a}", "${credentials.my-cred_1}"} {
		if !credentialRefPattern.MatchString(good) {
			t.Errorf("%q must be read as a credential binding", good)
		}
	}
}
