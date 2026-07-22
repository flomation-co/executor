// Package containerengine holds what every Oracle Cloud (OCI) Container Engine for
// Kubernetes (OKE) action shares: the API-signing-key credential, the single
// ContainerEngineClient factory, the resource summarisers, and the input/result helpers.
// Like the sibling OCI packages it has no Execute function, so the manifest generator skips
// it — but its category.go supplies the "Container Engine" sub-group.
//
// OKE is WORK-REQUEST based: create/update/delete on clusters, node pools, virtual node
// pools and add-ons return an opc-work-request-id header rather than the finished resource.
// Those actions return it via AsyncResult so a flow can poll Get Work Request until the
// operation completes (same fire-and-return shape as the Load Balancer node).
package containerengine

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	oke "github.com/oracle/oci-go-sdk/v65/containerengine"

	coreflow "flomation.app/automate/executor"
)

const (
	InputTenancyOCID     = "tenancy_ocid"
	InputUserOCID        = "user_ocid"
	InputRegion          = "region"
	InputFingerprint     = "fingerprint"
	InputPrivateKey      = "private_key"
	InputPassphrase      = "private_key_passphrase"
	InputCompartmentOCID = "compartment_ocid"
)

// ListMaxPages bounds every list action's pagination walk.
const ListMaxPages = 25

var validRegion = regexp.MustCompile(`^[a-z0-9-]+$`)

// Auth carries the API-signing-key material plus the compartment scope.
type Auth struct {
	TenancyOCID     string
	UserOCID        string
	Region          string
	Fingerprint     string
	privateKey      string
	passphrase      string
	CompartmentOCID string
	provider        common.ConfigurationProvider
}

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
		"tenancy OCID": a.TenancyOCID, "user OCID": a.UserOCID, "region": a.Region, "fingerprint": a.Fingerprint,
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

func (a Auth) RequiredCompartment() (string, error) {
	if a.CompartmentOCID == "" {
		return "", fmt.Errorf("compartment OCID is required")
	}
	return a.CompartmentOCID, nil
}

// Client builds the single ContainerEngineClient the whole node uses.
func Client(inputs []*coreflow.Connection) (auth Auth, client oke.ContainerEngineClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, oke.ContainerEngineClient{}, ErrorResult(err.Error())
	}
	c, err := oke.NewContainerEngineClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, oke.ContainerEngineClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
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

func fieldLabel(name string) string { return strings.ReplaceAll(name, "_", " ") }

func RequiredString(name string, inputs []*coreflow.Connection) (string, error) {
	v := strings.TrimSpace(OptionalString(name, inputs))
	if v == "" {
		return "", fmt.Errorf("%s is required", fieldLabel(name))
	}
	return v, nil
}

func OptionalBool(name string, inputs []*coreflow.Connection, def bool) bool {
	c := coreflow.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	raw := strings.ToLower(strings.TrimSpace(OptionalString(name, inputs)))
	if raw == "" {
		return def
	}
	return raw == "true" || raw == "yes" || raw == "1"
}

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

func RequiredInt(name string, inputs []*coreflow.Connection) (int, error) {
	n, ok, err := OptionalInt(name, inputs)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("%s is required", fieldLabel(name))
	}
	return n, nil
}

// InputStrings splits a comma-separated input into a trimmed, non-empty slice.
func InputStrings(name string, inputs []*coreflow.Connection) []string {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

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

func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func IntOrNil(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func FormatTime(t *common.SDKTime) string {
	if t == nil {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// Resource summarisers
// ---------------------------------------------------------------------------

func SummariseCluster(c *oke.Cluster) map[string]interface{} {
	out := map[string]interface{}{
		"id":                 Str(c.Id),
		"name":               Str(c.Name),
		"compartment_id":     Str(c.CompartmentId),
		"vcn_id":             Str(c.VcnId),
		"kubernetes_version": Str(c.KubernetesVersion),
		"lifecycle_state":    string(c.LifecycleState),
		"lifecycle_details":  Str(c.LifecycleDetails),
	}
	if c.Endpoints != nil {
		out["endpoints"] = map[string]interface{}{
			"kubernetes":         Str(c.Endpoints.Kubernetes),
			"public_endpoint":    Str(c.Endpoints.PublicEndpoint),
			"private_endpoint":   Str(c.Endpoints.PrivateEndpoint),
			"vcn_hostname":       Str(c.Endpoints.VcnHostnameEndpoint),
		}
	}
	return out
}

func SummariseClusterSummary(c *oke.ClusterSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(c.Id),
		"name":               Str(c.Name),
		"compartment_id":     Str(c.CompartmentId),
		"vcn_id":             Str(c.VcnId),
		"kubernetes_version": Str(c.KubernetesVersion),
		"lifecycle_state":    string(c.LifecycleState),
	}
}

func SummariseNodePool(n *oke.NodePool) map[string]interface{} {
	out := map[string]interface{}{
		"id":                 Str(n.Id),
		"name":               Str(n.Name),
		"compartment_id":     Str(n.CompartmentId),
		"cluster_id":         Str(n.ClusterId),
		"kubernetes_version": Str(n.KubernetesVersion),
		"node_shape":         Str(n.NodeShape),
		"lifecycle_state":    string(n.LifecycleState),
		"lifecycle_details":  Str(n.LifecycleDetails),
	}
	if n.NodeConfigDetails != nil {
		out["size"] = IntOrNil(n.NodeConfigDetails.Size)
	}
	nodes := make([]map[string]interface{}, 0, len(n.Nodes))
	for i := range n.Nodes {
		nodes = append(nodes, SummariseNode(&n.Nodes[i]))
	}
	out["nodes"] = nodes
	return out
}

func SummariseNodePoolSummary(n *oke.NodePoolSummary) map[string]interface{} {
	out := map[string]interface{}{
		"id":                 Str(n.Id),
		"name":               Str(n.Name),
		"compartment_id":     Str(n.CompartmentId),
		"cluster_id":         Str(n.ClusterId),
		"kubernetes_version": Str(n.KubernetesVersion),
		"node_shape":         Str(n.NodeShape),
		"lifecycle_state":    string(n.LifecycleState),
	}
	if n.NodeConfigDetails != nil {
		out["size"] = IntOrNil(n.NodeConfigDetails.Size)
	}
	return out
}

func SummariseNode(n *oke.Node) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(n.Id),
		"name":                Str(n.Name),
		"kubernetes_version":  Str(n.KubernetesVersion),
		"availability_domain": Str(n.AvailabilityDomain),
		"subnet_id":           Str(n.SubnetId),
		"node_pool_id":        Str(n.NodePoolId),
		"private_ip":          Str(n.PrivateIp),
		"public_ip":           Str(n.PublicIp),
		"lifecycle_state":     string(n.LifecycleState),
	}
}

func SummariseVirtualNodePool(v *oke.VirtualNodePool) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(v.Id),
		"display_name":       Str(v.DisplayName),
		"compartment_id":     Str(v.CompartmentId),
		"cluster_id":         Str(v.ClusterId),
		"kubernetes_version": Str(v.KubernetesVersion),
		"size":               IntOrNil(v.Size),
		"lifecycle_state":    string(v.LifecycleState),
		"lifecycle_details":  Str(v.LifecycleDetails),
		"time_created":       FormatTime(v.TimeCreated),
	}
}

func SummariseVirtualNodePoolSummary(v *oke.VirtualNodePoolSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(v.Id),
		"display_name":       Str(v.DisplayName),
		"compartment_id":     Str(v.CompartmentId),
		"cluster_id":         Str(v.ClusterId),
		"kubernetes_version": Str(v.KubernetesVersion),
		"size":               IntOrNil(v.Size),
		"lifecycle_state":    string(v.LifecycleState),
	}
}

func SummariseVirtualNode(v *oke.VirtualNode) map[string]interface{} {
	return map[string]interface{}{
		"id":                   Str(v.Id),
		"display_name":         Str(v.DisplayName),
		"virtual_node_pool_id": Str(v.VirtualNodePoolId),
		"availability_domain":  Str(v.AvailabilityDomain),
		"subnet_id":            Str(v.SubnetId),
		"private_ip":           Str(v.PrivateIp),
		"lifecycle_state":      string(v.LifecycleState),
	}
}

func SummariseVirtualNodeSummary(v *oke.VirtualNodeSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                   Str(v.Id),
		"display_name":         Str(v.DisplayName),
		"virtual_node_pool_id": Str(v.VirtualNodePoolId),
		"availability_domain":  Str(v.AvailabilityDomain),
		"private_ip":           Str(v.PrivateIp),
		"lifecycle_state":      string(v.LifecycleState),
	}
}

func SummariseAddon(a *oke.Addon) map[string]interface{} {
	return map[string]interface{}{
		"name":                      Str(a.Name),
		"version":                   Str(a.Version),
		"current_installed_version": Str(a.CurrentInstalledVersion),
		"lifecycle_state":           string(a.LifecycleState),
		"time_created":              FormatTime(a.TimeCreated),
	}
}

func SummariseAddonSummary(a *oke.AddonSummary) map[string]interface{} {
	return map[string]interface{}{
		"name":                      Str(a.Name),
		"version":                   Str(a.Version),
		"current_installed_version": Str(a.CurrentInstalledVersion),
		"lifecycle_state":           string(a.LifecycleState),
		"time_created":              FormatTime(a.TimeCreated),
	}
}

func SummariseWorkloadMapping(w *oke.WorkloadMapping) map[string]interface{} {
	return map[string]interface{}{
		"id":                    Str(w.Id),
		"cluster_id":            Str(w.ClusterId),
		"namespace":             Str(w.Namespace),
		"mapped_tenancy_id":     Str(w.MappedTenancyId),
		"mapped_compartment_id": Str(w.MappedCompartmentId),
		"lifecycle_state":       string(w.LifecycleState),
		"time_created":          FormatTime(w.TimeCreated),
	}
}

func SummariseWorkloadMappingSummary(w *oke.WorkloadMappingSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                    Str(w.Id),
		"cluster_id":            Str(w.ClusterId),
		"namespace":             Str(w.Namespace),
		"mapped_tenancy_id":     Str(w.MappedTenancyId),
		"mapped_compartment_id": Str(w.MappedCompartmentId),
		"lifecycle_state":       string(w.LifecycleState),
	}
}

func workRequestResources(rs []oke.WorkRequestResource) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(rs))
	for i := range rs {
		out = append(out, map[string]interface{}{
			"entity_type": Str(rs[i].EntityType),
			"action_type": string(rs[i].ActionType),
			"identifier":  Str(rs[i].Identifier),
			"entity_uri":  Str(rs[i].EntityUri),
		})
	}
	return out
}

func SummariseWorkRequest(w *oke.WorkRequest) map[string]interface{} {
	return map[string]interface{}{
		"id":             Str(w.Id),
		"operation_type": string(w.OperationType),
		"status":         string(w.Status),
		"compartment_id": Str(w.CompartmentId),
		"resources":      workRequestResources(w.Resources),
		"time_accepted":  FormatTime(w.TimeAccepted),
		"time_started":   FormatTime(w.TimeStarted),
		"time_finished":  FormatTime(w.TimeFinished),
	}
}

func SummariseWorkRequestSummary(w *oke.WorkRequestSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":             Str(w.Id),
		"operation_type": string(w.OperationType),
		"status":         string(w.Status),
		"compartment_id": Str(w.CompartmentId),
		"time_accepted":  FormatTime(w.TimeAccepted),
		"time_started":   FormatTime(w.TimeStarted),
		"time_finished":  FormatTime(w.TimeFinished),
	}
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

// Result is the standard success envelope for synchronous reads.
func Result(msg string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{"tool_result": msg, "success": true}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// AsyncResult is the envelope for a fire-and-return work-request operation: the caller
// polls Get Work Request with work_request_id until it completes.
func AsyncResult(msg, workRequestID string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result":     msg,
		"work_request_id": workRequestID,
		"success":         true,
	}
}

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{"success": false, "error": msg, "tool_result": msg}
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

func Context() context.Context { return context.Background() }
