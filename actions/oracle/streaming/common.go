// Package streaming holds what every Oracle Cloud (OCI) Streaming action shares: the
// API-signing-key credential, the two clients this service spans, the resource summarisers,
// and the input/result helpers. Like the sibling OCI packages it has no Execute function, so
// the manifest generator skips it — but its category.go supplies the "Streaming" sub-group.
//
// OCI Streaming has two planes with DIFFERENT endpoints:
//   - StreamAdminClient — the REGIONAL control plane: streams, stream pools and connect
//     harnesses (create/get/list/update/delete/move) plus work requests.
//   - StreamClient — the DATA plane: put/get messages, cursors and consumer groups. Crucially
//     this client is NOT regional — every stream publishes its own `messagesEndpoint`, and the
//     data client must be built against THAT host (mirrors the per-vault-endpoint quirk in the
//     Vault node). DataPlaneClientForStream resolves it automatically from the stream OCID so
//     operators only ever supply the stream, never a raw endpoint URL.
package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/streaming"

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

// ---------------------------------------------------------------------------
// Client preambles
// ---------------------------------------------------------------------------

// AdminClient is the regional control-plane client (streams, pools, connect harnesses).
func AdminClient(inputs []*coreflow.Connection) (auth Auth, client streaming.StreamAdminClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, streaming.StreamAdminClient{}, ErrorResult(err.Error())
	}
	c, err := streaming.NewStreamAdminClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, streaming.StreamAdminClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// DataPlaneClientForStream builds the data-plane client for one stream. The data plane lives on
// the stream's OWN messagesEndpoint (not the region), so we resolve it from the stream OCID via
// the admin client's GetStream, then point a StreamClient at that host. This keeps every
// data-plane action operator-simple: the user supplies the stream, never a raw endpoint. A
// stream that is not yet ACTIVE is rejected with a clear message rather than a signing failure.
func DataPlaneClientForStream(inputs []*coreflow.Connection, streamID string) (auth Auth, client streaming.StreamClient, errResult map[string]interface{}) {
	a, admin, errRes := AdminClient(inputs)
	if errRes != nil {
		return Auth{}, streaming.StreamClient{}, errRes
	}
	if strings.TrimSpace(streamID) == "" {
		return Auth{}, streaming.StreamClient{}, ErrorResult("stream OCID is required")
	}
	resp, err := admin.GetStream(Context(), streaming.GetStreamRequest{StreamId: &streamID})
	if err != nil {
		return Auth{}, streaming.StreamClient{}, ErrorResult(a.OCIError(err))
	}
	endpoint := Str(resp.MessagesEndpoint)
	if endpoint == "" {
		return Auth{}, streaming.StreamClient{}, ErrorResult(fmt.Sprintf("stream %q has no messages endpoint yet (it is %s) — wait until it is ACTIVE", Str(resp.Name), string(resp.LifecycleState)))
	}
	c, err := streaming.NewStreamClientWithConfigurationProvider(a.provider, endpoint)
	if err != nil {
		return Auth{}, streaming.StreamClient{}, ErrorResult(a.OCIError(err))
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

func fieldLabel(name string) string {
	return strings.ReplaceAll(name, "_", " ")
}

func RequiredString(name string, inputs []*coreflow.Connection) (string, error) {
	v := strings.TrimSpace(OptionalString(name, inputs))
	if v == "" {
		return "", fmt.Errorf("%s is required", fieldLabel(name))
	}
	return v, nil
}

// OptionalInt reads an integer input. ok is false when the field is blank (leave it unset); an
// explicit value — even 0 — sets ok true so a deliberate 0 is distinguishable from "not given".
func OptionalInt(name string, inputs []*coreflow.Connection) (val int, ok bool, err error) {
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

// RequiredInt reads a mandatory integer input.
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

func OptionalBool(name string, inputs []*coreflow.Connection) bool {
	c := coreflow.FindConnection(name, inputs)
	if c == nil {
		return false
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	return strings.EqualFold(strings.TrimSpace(OptionalString(name, inputs)), "true")
}

func FreeformTags(name string, inputs []*coreflow.Connection) (map[string]string, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var flat map[string]string
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return nil, fmt.Errorf("freeform tags must be a JSON object of string values, e.g. {\"env\":\"prod\"}: %s", err.Error())
	}
	return flat, nil
}

func DefinedTags(name string, inputs []*coreflow.Connection) (map[string]map[string]interface{}, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var nested map[string]map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &nested); err != nil {
		return nil, fmt.Errorf("defined tags must be a JSON object of namespaces, e.g. {\"Ops\":{\"env\":\"prod\"}}: %s", err.Error())
	}
	return nested, nil
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

func Int64OrNil(p *int64) interface{} {
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

func SummariseStream(s *streaming.Stream) map[string]interface{} {
	return map[string]interface{}{
		"id":                Str(s.Id),
		"name":              Str(s.Name),
		"partitions":        IntOrNil(s.Partitions),
		"retention_hours":   IntOrNil(s.RetentionInHours),
		"compartment_id":    Str(s.CompartmentId),
		"stream_pool_id":    Str(s.StreamPoolId),
		"lifecycle_state":   string(s.LifecycleState),
		"messages_endpoint": Str(s.MessagesEndpoint),
		"time_created":      FormatTime(s.TimeCreated),
	}
}

func SummariseStreamSummary(s *streaming.StreamSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                Str(s.Id),
		"name":              Str(s.Name),
		"partitions":        IntOrNil(s.Partitions),
		"compartment_id":    Str(s.CompartmentId),
		"stream_pool_id":    Str(s.StreamPoolId),
		"lifecycle_state":   string(s.LifecycleState),
		"messages_endpoint": Str(s.MessagesEndpoint),
		"time_created":      FormatTime(s.TimeCreated),
	}
}

func SummariseStreamPool(p *streaming.StreamPool) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(p.Id),
		"name":            Str(p.Name),
		"compartment_id":  Str(p.CompartmentId),
		"lifecycle_state": string(p.LifecycleState),
		"is_private":      p.IsPrivate != nil && *p.IsPrivate,
		"endpoint_fqdn":   Str(p.EndpointFqdn),
		"time_created":    FormatTime(p.TimeCreated),
	}
}

func SummariseStreamPoolSummary(p *streaming.StreamPoolSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(p.Id),
		"name":            Str(p.Name),
		"compartment_id":  Str(p.CompartmentId),
		"lifecycle_state": string(p.LifecycleState),
		"is_private":      p.IsPrivate != nil && *p.IsPrivate,
		"time_created":    FormatTime(p.TimeCreated),
	}
}

func SummariseConnectHarness(h *streaming.ConnectHarness) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(h.Id),
		"name":            Str(h.Name),
		"compartment_id":  Str(h.CompartmentId),
		"lifecycle_state": string(h.LifecycleState),
		"time_created":    FormatTime(h.TimeCreated),
	}
}

func SummariseConnectHarnessSummary(h *streaming.ConnectHarnessSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(h.Id),
		"name":            Str(h.Name),
		"compartment_id":  Str(h.CompartmentId),
		"lifecycle_state": string(h.LifecycleState),
		"time_created":    FormatTime(h.TimeCreated),
	}
}

func SummariseWorkRequest(w *streaming.WorkRequest) map[string]interface{} {
	out := map[string]interface{}{
		"id":               Str(w.Id),
		"operation_type":   string(w.OperationType),
		"status":           string(w.Status),
		"compartment_id":   Str(w.CompartmentId),
		"percent_complete": w.PercentComplete,
		"time_accepted":    FormatTime(w.TimeAccepted),
		"time_started":     FormatTime(w.TimeStarted),
		"time_finished":    FormatTime(w.TimeFinished),
	}
	var resources []map[string]interface{}
	for _, r := range w.Resources {
		resources = append(resources, map[string]interface{}{
			"entity_type": Str(r.EntityType),
			"action_type": string(r.ActionType),
			"identifier":  Str(r.Identifier),
			"entity_uri":  Str(r.EntityUri),
		})
	}
	out["resources"] = resources
	return out
}

// SummariseMessage shapes one consumed message. Key and Value are raw bytes on the wire (the SDK
// base64-decodes them for us); we surface them as UTF-8 strings, which is what operators publish.
func SummariseMessage(m *streaming.Message) map[string]interface{} {
	return map[string]interface{}{
		"stream":    Str(m.Stream),
		"partition": Str(m.Partition),
		"key":       string(m.Key),
		"value":     string(m.Value),
		"offset":    Int64OrNil(m.Offset),
		"timestamp": FormatTime(m.Timestamp),
	}
}

// SummarisePutResult shapes one entry of a PutMessages response.
func SummarisePutResult(e *streaming.PutMessagesResultEntry) map[string]interface{} {
	return map[string]interface{}{
		"partition":     Str(e.Partition),
		"offset":        Int64OrNil(e.Offset),
		"timestamp":     FormatTime(e.Timestamp),
		"error":         Str(e.Error),
		"error_message": Str(e.ErrorMessage),
	}
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

// Result is the standard success envelope: tool_result + success plus any extra keys.
func Result(msg string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{"tool_result": msg, "success": true}
	for k, v := range extra {
		out[k] = v
	}
	return out
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
