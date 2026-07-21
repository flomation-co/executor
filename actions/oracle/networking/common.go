// Package networking holds what every Oracle Cloud (OCI) Networking action shares:
// the API-signing-key credential, the VirtualNetwork service client factory, a
// generalized per-resource client preamble, the resource summarisers, and the
// structured JSON rule decoders. Like the sibling OCI packages it has no Execute
// function, so the manifest generator skips it — but its category.go supplies the
// "Networking" sub-category.
//
// The auth block is the OCI signing-key model, identical to oracle/compute,
// oracle/objectstorage and oracle/autonomousdatabase. Every networking resource
// (VCN, subnet, security list, route table, gateways, NSG, DHCP options, public IP)
// lives on the SAME core.VirtualNetworkClient, so this package generalizes the
// per-resource preamble (NetworkResourceClient) rather than hard-coding one helper
// per resource type. As with the siblings, the manifest generator only resolves
// INLINE Inputs literals, so the credential + compartment input *declarations* must
// still be copy-pasted into each action's Inputs.
package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	coreflow "flomation.app/automate/executor"
)

// Standard input names shared by every OCI Networking action.
const (
	InputTenancyOCID     = "tenancy_ocid"
	InputUserOCID        = "user_ocid"
	InputRegion          = "region"
	InputFingerprint     = "fingerprint"
	InputPrivateKey      = "private_key"
	InputPassphrase      = "private_key_passphrase"
	InputCompartmentOCID = "compartment_ocid"
)

// ListMaxPages bounds every list action's pagination walk so one node run can't
// turn into an unbounded sequence of API calls; a walk that hits the cap sets
// truncated=true so a capped result is distinguishable from a complete one.
const ListMaxPages = 25

// validRegion constrains the host-selecting region to a plain label (see GetAuth).
var validRegion = regexp.MustCompile(`^[a-z0-9-]+$`)

// Auth carries the API-signing-key material plus the compartment scope that
// list/create calls need. Per-resource ops are scoped by the resource OCID.
type Auth struct {
	TenancyOCID     string
	UserOCID        string
	Region          string
	Fingerprint     string
	privateKey      string // PEM — never echoed (see redact)
	passphrase      string
	CompartmentOCID string
	provider        common.ConfigurationProvider
}

// GetAuth reads the standard credential block and builds the signing-key
// ConfigurationProvider, validating the host-selecting region and eagerly parsing
// the PEM so a bad key fails with a clean message up front.
func GetAuth(inputs []*coreflow.Connection) (Auth, error) {
	a := Auth{
		TenancyOCID:     strings.TrimSpace(OptionalString(InputTenancyOCID, inputs)),
		UserOCID:        strings.TrimSpace(OptionalString(InputUserOCID, inputs)),
		Region:          strings.ToLower(strings.TrimSpace(OptionalString(InputRegion, inputs))),
		Fingerprint:     strings.TrimSpace(OptionalString(InputFingerprint, inputs)),
		privateKey:      OptionalString(InputPrivateKey, inputs),
		passphrase:      OptionalString(InputPassphrase, inputs),
		CompartmentOCID: strings.TrimSpace(OptionalString(InputCompartmentOCID, inputs)),
	}
	for field, val := range map[string]string{
		"tenancy OCID": a.TenancyOCID,
		"user OCID":    a.UserOCID,
		"region":       a.Region,
		"fingerprint":  a.Fingerprint,
	} {
		if val == "" {
			return Auth{}, fmt.Errorf("%s is required", field)
		}
	}
	if !validRegion.MatchString(a.Region) {
		return Auth{}, fmt.Errorf("region %q is not a valid OCI region (expected a plain identifier like uk-london-1)", a.Region)
	}
	if strings.TrimSpace(a.privateKey) == "" {
		return Auth{}, fmt.Errorf("private key (PEM) is required")
	}
	var pass *string
	if a.passphrase != "" {
		pass = &a.passphrase
	}
	a.provider = common.NewRawConfigurationProvider(a.TenancyOCID, a.UserOCID, a.Region, a.Fingerprint, a.privateKey, pass)
	if _, err := a.provider.PrivateRSAKey(); err != nil {
		return Auth{}, fmt.Errorf("private key could not be parsed — check it is the full PEM (and the passphrase, if set): %s", a.redact(err.Error()))
	}
	return a, nil
}

// NetworkClient builds an authenticated OCI VirtualNetwork client — the single
// service that owns every networking resource.
func (a Auth) NetworkClient() (core.VirtualNetworkClient, error) {
	return core.NewVirtualNetworkClientWithConfigurationProvider(a.provider)
}

// NetworkResourceClient reads the credential block + one resource-OCID input
// (named by ocidInputName, e.g. "vcn_ocid", "subnet_ocid", "nsg_ocid") and builds
// the VirtualNetwork client — the shared preamble for every per-resource action.
// Since all networking resources share one client, the OCID input is parameterized
// rather than hard-coded. On any setup error it returns a ready ErrorResult so the
// caller can `return errResult, nil`; on success errResult is nil.
func NetworkResourceClient(inputs []*coreflow.Connection, ocidInputName string) (auth Auth, client core.VirtualNetworkClient, ocid string, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, core.VirtualNetworkClient{}, "", ErrorResult(err.Error())
	}
	id, err := RequiredString(ocidInputName, inputs)
	if err != nil {
		return Auth{}, core.VirtualNetworkClient{}, "", ErrorResult(err.Error())
	}
	c, err := a.NetworkClient()
	if err != nil {
		return Auth{}, core.VirtualNetworkClient{}, "", ErrorResult(a.OCIError(err))
	}
	return a, c, id, nil
}

// RequiredCompartment names the field when a compartment-scoped action is missing it.
func (a Auth) RequiredCompartment() (string, error) {
	if a.CompartmentOCID == "" {
		return "", fmt.Errorf("compartment OCID is required")
	}
	return a.CompartmentOCID, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

func OptionalString(name string, inputs []*coreflow.Connection) string {
	c := coreflow.FindConnection(name, inputs)
	if c == nil {
		return ""
	}
	if s := c.String(); s != nil {
		return *s
	}
	return ""
}

func fieldLabel(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "_", " "), "ocid", "OCID")
}

func RequiredString(name string, inputs []*coreflow.Connection) (string, error) {
	if v := strings.TrimSpace(OptionalString(name, inputs)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s is required", fieldLabel(name))
}

func OptionalBool(name string, inputs []*coreflow.Connection, def bool) bool {
	c := coreflow.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	return def
}

// BoolWasSet reports whether the operator provided any value for a boolean input
// (so an action can distinguish "unset" from "explicitly false").
func BoolWasSet(name string, inputs []*coreflow.Connection) bool {
	return strings.TrimSpace(OptionalString(name, inputs)) != ""
}

func OptionalInt(name string, inputs []*coreflow.Connection) (int, bool, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return 0, false, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be a whole number", fieldLabel(name))
	}
	return n, true, nil
}

// InputStrings splits a comma-separated input into a trimmed, non-empty slice
// (used for CIDR blocks, security-list ids on a subnet, etc.).
func InputStrings(name string, inputs []*coreflow.Connection) []string {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// FreeformTags parses a JSON object input into map[string]string; blank → nil.
func FreeformTags(name string, inputs []*coreflow.Connection) (map[string]string, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var flat map[string]string
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return nil, fmt.Errorf("tags must be a JSON object of string values, e.g. {\"env\":\"prod\"}: %s", err.Error())
	}
	return flat, nil
}

// ---------------------------------------------------------------------------
// Structured JSON rule decoders
//
// Security-list ingress/egress rules, route rules and NSG rules are modelled as
// JSON arrays that decode straight into the SDK structs (which carry the matching
// json tags), so operators supply e.g.
//   [{"protocol":"6","source":"0.0.0.0/0","tcpOptions":{"destinationPortRange":{"min":443,"max":443}}}]
// ---------------------------------------------------------------------------

func DecodeIngressRules(name string, inputs []*coreflow.Connection) ([]core.IngressSecurityRule, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var rules []core.IngressSecurityRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of ingress rules (each with at least protocol + source): %s", fieldLabel(name), err.Error())
	}
	return rules, nil
}

func DecodeEgressRules(name string, inputs []*coreflow.Connection) ([]core.EgressSecurityRule, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var rules []core.EgressSecurityRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of egress rules (each with at least protocol + destination): %s", fieldLabel(name), err.Error())
	}
	return rules, nil
}

func DecodeRouteRules(name string, inputs []*coreflow.Connection) ([]core.RouteRule, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var rules []core.RouteRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of route rules (each with networkEntityId + destination): %s", fieldLabel(name), err.Error())
	}
	return rules, nil
}

func DecodeNsgAddRules(name string, inputs []*coreflow.Connection) ([]core.AddSecurityRuleDetails, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var rules []core.AddSecurityRuleDetails
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of NSG rules (each with direction + protocol): %s", fieldLabel(name), err.Error())
	}
	return rules, nil
}

func DecodeNsgUpdateRules(name string, inputs []*coreflow.Connection) ([]core.UpdateSecurityRuleDetails, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var rules []core.UpdateSecurityRuleDetails
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of NSG rule updates (each with the rule id): %s", fieldLabel(name), err.Error())
	}
	return rules, nil
}

// DecodeDhcpOptions decodes the operator JSON into the polymorphic []core.DhcpOption
// slice. Because DhcpOption is an interface, each element is read as a generic
// object and dispatched on its "type" ("DomainNameServer" | "SearchDomain"). Blank
// input yields nil (so an update leaves the existing options untouched).
func DecodeDhcpOptions(name string, inputs []*coreflow.Connection) ([]core.DhcpOption, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var elems []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &elems); err != nil {
		return nil, fmt.Errorf(`%s must be a JSON array of option objects, each with a "type" of "DomainNameServer" or "SearchDomain": %s`, fieldLabel(name), err.Error())
	}
	options := make([]core.DhcpOption, 0, len(elems))
	for i, m := range elems {
		typ, _ := m["type"].(string)
		typ = strings.TrimSpace(typ)
		switch typ {
		case "DomainNameServer":
			serverType, _ := m["serverType"].(string)
			serverType = strings.TrimSpace(serverType)
			if serverType == "" {
				return nil, fmt.Errorf(`DHCP option #%d (DomainNameServer) requires "serverType" (one of VcnLocal, VcnLocalPlusInternet, CustomDnsServer)`, i+1)
			}
			enum, ok := core.GetMappingDhcpDnsOptionServerTypeEnum(serverType)
			if !ok {
				return nil, fmt.Errorf(`DHCP option #%d (DomainNameServer) has unsupported serverType %q (use VcnLocal, VcnLocalPlusInternet or CustomDnsServer)`, i+1, serverType)
			}
			options = append(options, core.DhcpDnsOption{ServerType: enum, CustomDnsServers: jsonStringSlice(m["customDnsServers"])})
		case "SearchDomain":
			names := jsonStringSlice(m["searchDomainNames"])
			if len(names) == 0 {
				return nil, fmt.Errorf(`DHCP option #%d (SearchDomain) requires a non-empty "searchDomainNames" array (e.g. ["example.com"])`, i+1)
			}
			options = append(options, core.DhcpSearchDomainOption{SearchDomainNames: names})
		case "":
			return nil, fmt.Errorf(`DHCP option #%d is missing its "type" (must be "DomainNameServer" or "SearchDomain")`, i+1)
		default:
			return nil, fmt.Errorf(`DHCP option #%d has unknown type %q (must be "DomainNameServer" or "SearchDomain")`, i+1, typ)
		}
	}
	return options, nil
}

// jsonStringSlice coerces a decoded JSON value into a []string, dropping non-string
// members; a nil/non-array value yields nil.
func jsonStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func StringPtr(s string) *string { return &s }

func FormatTime(t *common.SDKTime) string {
	if t == nil {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// Resource summarisers — one per resource type so the output shape is identical
// across each resource's create/get/get_all actions. netBase carries the five keys
// every networking resource shares; each summariser adds its resource-specific
// fields.
// ---------------------------------------------------------------------------

func netBase(id, displayName, lifecycleState, compartmentID string, timeCreated *common.SDKTime) map[string]interface{} {
	return map[string]interface{}{
		"id":              id,
		"display_name":    displayName,
		"lifecycle_state": lifecycleState,
		"compartment_id":  compartmentID,
		"time_created":    FormatTime(timeCreated),
	}
}

func SummariseVcn(v *core.Vcn) map[string]interface{} {
	m := netBase(Str(v.Id), Str(v.DisplayName), string(v.LifecycleState), Str(v.CompartmentId), v.TimeCreated)
	m["cidr_blocks"] = v.CidrBlocks
	m["dns_label"] = Str(v.DnsLabel)
	m["default_route_table_id"] = Str(v.DefaultRouteTableId)
	m["default_security_list_id"] = Str(v.DefaultSecurityListId)
	m["default_dhcp_options_id"] = Str(v.DefaultDhcpOptionsId)
	return m
}

func SummariseSubnet(s *core.Subnet) map[string]interface{} {
	m := netBase(Str(s.Id), Str(s.DisplayName), string(s.LifecycleState), Str(s.CompartmentId), s.TimeCreated)
	m["vcn_id"] = Str(s.VcnId)
	m["cidr_block"] = Str(s.CidrBlock)
	m["availability_domain"] = Str(s.AvailabilityDomain)
	m["route_table_id"] = Str(s.RouteTableId)
	m["dns_label"] = Str(s.DnsLabel)
	m["security_list_ids"] = s.SecurityListIds
	m["prohibit_public_ip_on_vnic"] = s.ProhibitPublicIpOnVnic != nil && *s.ProhibitPublicIpOnVnic
	return m
}

func SummariseSecurityList(s *core.SecurityList) map[string]interface{} {
	m := netBase(Str(s.Id), Str(s.DisplayName), string(s.LifecycleState), Str(s.CompartmentId), s.TimeCreated)
	m["vcn_id"] = Str(s.VcnId)
	m["ingress_security_rules"] = s.IngressSecurityRules
	m["egress_security_rules"] = s.EgressSecurityRules
	return m
}

func SummariseRouteTable(r *core.RouteTable) map[string]interface{} {
	m := netBase(Str(r.Id), Str(r.DisplayName), string(r.LifecycleState), Str(r.CompartmentId), r.TimeCreated)
	m["vcn_id"] = Str(r.VcnId)
	m["route_rules"] = r.RouteRules
	return m
}

func SummariseInternetGateway(g *core.InternetGateway) map[string]interface{} {
	m := netBase(Str(g.Id), Str(g.DisplayName), string(g.LifecycleState), Str(g.CompartmentId), g.TimeCreated)
	m["vcn_id"] = Str(g.VcnId)
	m["is_enabled"] = g.IsEnabled != nil && *g.IsEnabled
	m["route_table_id"] = Str(g.RouteTableId)
	return m
}

func SummariseNatGateway(g *core.NatGateway) map[string]interface{} {
	m := netBase(Str(g.Id), Str(g.DisplayName), string(g.LifecycleState), Str(g.CompartmentId), g.TimeCreated)
	m["vcn_id"] = Str(g.VcnId)
	m["block_traffic"] = g.BlockTraffic != nil && *g.BlockTraffic
	m["nat_ip"] = Str(g.NatIp)
	m["route_table_id"] = Str(g.RouteTableId)
	return m
}

func SummariseServiceGateway(g *core.ServiceGateway) map[string]interface{} {
	m := netBase(Str(g.Id), Str(g.DisplayName), string(g.LifecycleState), Str(g.CompartmentId), g.TimeCreated)
	m["vcn_id"] = Str(g.VcnId)
	m["block_traffic"] = g.BlockTraffic != nil && *g.BlockTraffic
	m["route_table_id"] = Str(g.RouteTableId)
	m["services"] = g.Services
	return m
}

func SummariseNsg(n *core.NetworkSecurityGroup) map[string]interface{} {
	m := netBase(Str(n.Id), Str(n.DisplayName), string(n.LifecycleState), Str(n.CompartmentId), n.TimeCreated)
	m["vcn_id"] = Str(n.VcnId)
	return m
}

func SummariseDhcpOptions(d *core.DhcpOptions) map[string]interface{} {
	m := netBase(Str(d.Id), Str(d.DisplayName), string(d.LifecycleState), Str(d.CompartmentId), d.TimeCreated)
	m["vcn_id"] = Str(d.VcnId)
	m["options"] = d.Options
	return m
}

func SummarisePublicIp(p *core.PublicIp) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(p.Id),
		"display_name":        Str(p.DisplayName),
		"lifecycle_state":     string(p.LifecycleState),
		"compartment_id":      Str(p.CompartmentId),
		"ip_address":          Str(p.IpAddress),
		"lifetime":            string(p.Lifetime),
		"private_ip_id":       Str(p.PrivateIpId),
		"availability_domain": Str(p.AvailabilityDomain),
		"time_created":        FormatTime(p.TimeCreated),
	}
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{"success": false, "error": msg, "tool_result": msg}
}

func ServiceErrorCode(err error) (string, int) {
	if se, ok := common.IsServiceError(err); ok {
		return se.GetCode(), se.GetHTTPStatusCode()
	}
	return "", 0
}

func (a Auth) OCIError(err error) string {
	if err == nil {
		return ""
	}
	if se, ok := common.IsServiceError(err); ok {
		code := se.GetCode()
		if code == "" {
			code = fmt.Sprintf("HTTP %d", se.GetHTTPStatusCode())
		}
		return a.redact(fmt.Sprintf("OCI rejected the request (%s): %s", code, se.GetMessage()))
	}
	return a.redact(firstLine(err.Error()))
}

func (a Auth) redact(s string) string {
	if k := strings.TrimSpace(a.privateKey); k != "" {
		s = strings.ReplaceAll(s, k, "REDACTED")
		s = strings.ReplaceAll(s, a.privateKey, "REDACTED")
	}
	if a.passphrase != "" {
		s = strings.ReplaceAll(s, a.passphrase, "REDACTED")
	}
	return s
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// Context is what every action passes: no request deadline from the executor.
func Context() context.Context { return context.Background() }
