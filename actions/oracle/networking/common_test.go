package networking

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

func sc(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

func TestValidRegion(t *testing.T) {
	for _, ok := range []string{"uk-london-1", "us-ashburn-1", "eu-frankfurt-1"} {
		if !validRegion.MatchString(ok) {
			t.Errorf("validRegion rejected a real region %q", ok)
		}
	}
	for _, bad := range []string{"attacker.com", "uk-london-1.evil.com", "host:1337", "a/b", "UK-LONDON-1", ""} {
		if validRegion.MatchString(bad) {
			t.Errorf("validRegion ACCEPTED a host-injection region %q", bad)
		}
	}
}

func TestGetAuthRejectsBadRegion(t *testing.T) {
	in := []*core.Connection{
		sc("tenancy_ocid", "ocid1.tenancy.oc1..aaaa"), sc("user_ocid", "ocid1.user.oc1..aaaa"),
		sc("region", "attacker.com"), sc("fingerprint", "aa:bb:cc"),
		sc("private_key", "-----BEGIN RSA PRIVATE KEY-----\nx\n-----END RSA PRIVATE KEY-----"),
	}
	if _, err := GetAuth(in); err == nil || !strings.Contains(err.Error(), "region") {
		t.Fatalf("GetAuth should reject region \"attacker.com\", got %v", err)
	}
}

func TestGetAuthRedactsMalformedKey(t *testing.T) {
	const secret = "SUPERSECRETKEYBODY0123456789abcdef"
	in := []*core.Connection{
		sc("tenancy_ocid", "ocid1.tenancy.oc1..aaaa"), sc("user_ocid", "ocid1.user.oc1..aaaa"),
		sc("region", "uk-london-1"), sc("fingerprint", "aa:bb:cc"),
		sc("private_key", "-----BEGIN RSA PRIVATE KEY-----\n"+secret+"\n-----END RSA PRIVATE KEY-----"),
	}
	_, err := GetAuth(in)
	if err == nil {
		t.Fatal("GetAuth accepted a malformed private key")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("GetAuth leaked key material: %q", err)
	}
	res := ErrorResult(err.Error())
	for _, k := range []string{"error", "tool_result"} {
		if s, _ := res[k].(string); strings.Contains(s, secret) {
			t.Errorf("ErrorResult[%q] leaked key material", k)
		}
	}
}

func TestServiceErrorCode(t *testing.T) {
	if code, status := ServiceErrorCode(nil); code != "" || status != 0 {
		t.Errorf("ServiceErrorCode(nil) = (%q,%d), want (\"\",0)", code, status)
	}
}

func TestInputStrings(t *testing.T) {
	got := InputStrings("cidr_blocks", []*core.Connection{sc("cidr_blocks", " 10.0.0.0/16 , 10.1.0.0/16 ,")})
	if len(got) != 2 || got[0] != "10.0.0.0/16" || got[1] != "10.1.0.0/16" {
		t.Errorf("InputStrings split/trim wrong: %#v", got)
	}
	if InputStrings("x", []*core.Connection{sc("x", "")}) != nil {
		t.Error("blank should yield nil")
	}
}

// TestRuleDecoders covers the novel structured JSON rule decoders: valid JSON
// unmarshals into the SDK slice types, blank yields nil, malformed errors.
func TestRuleDecoders(t *testing.T) {
	in := func(name, v string) []*core.Connection { return []*core.Connection{sc(name, v)} }

	ing, err := DecodeIngressRules("ingress_security_rules", in("ingress_security_rules", `[{"protocol":"6","source":"0.0.0.0/0"}]`))
	if err != nil || len(ing) != 1 || Str(ing[0].Protocol) != "6" || Str(ing[0].Source) != "0.0.0.0/0" {
		t.Errorf("DecodeIngressRules = (%v, %v)", ing, err)
	}
	if r, err := DecodeIngressRules("x", in("x", "")); err != nil || r != nil {
		t.Errorf("blank ingress should be (nil,nil), got (%v,%v)", r, err)
	}
	if _, err := DecodeIngressRules("x", in("x", `[{"protocol":`)); err == nil {
		t.Error("malformed ingress JSON should error")
	}

	rr, err := DecodeRouteRules("route_rules", in("route_rules", `[{"networkEntityId":"ocid1.igw","destination":"0.0.0.0/0","destinationType":"CIDR_BLOCK"}]`))
	if err != nil || len(rr) != 1 || Str(rr[0].NetworkEntityId) != "ocid1.igw" {
		t.Errorf("DecodeRouteRules = (%v, %v)", rr, err)
	}

	nr, err := DecodeNsgAddRules("security_rules", in("security_rules", `[{"direction":"INGRESS","protocol":"6","source":"10.0.0.0/16"}]`))
	if err != nil || len(nr) != 1 || string(nr[0].Direction) != "INGRESS" {
		t.Errorf("DecodeNsgAddRules = (%v, %v)", nr, err)
	}
}
