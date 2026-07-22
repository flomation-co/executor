// Package queue holds what every Oracle Cloud (OCI) Queue action shares: the API-signing-key
// credential, the two clients this service spans, the resource summarisers, and the input/result
// helpers. Like the sibling OCI packages it has no Execute function, so the manifest generator
// skips it — but its category.go supplies the "Queue" sub-group.
//
// OCI Queue has two planes with DIFFERENT endpoints:
//   - QueueAdminClient — the REGIONAL control plane: queues
//     (create/get/list/update/delete/move/purge) plus work requests.
//   - QueueClient — the DATA plane: put/get/delete/update messages, stats, channels. Each queue
//     publishes its OWN messagesEndpoint, and the data client must target THAT host (mirrors the
//     per-stream endpoint in the Streaming node and the per-vault endpoint in Vault).
//     DataPlaneClientForQueue resolves it automatically from the queue OCID.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/queue"

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

const ListMaxPages = 25

var validRegion = regexp.MustCompile(`^[a-z0-9-]+$`)

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

// AdminClient is the regional control-plane client (queues, consumer groups, work requests).
func AdminClient(inputs []*coreflow.Connection) (auth Auth, client queue.QueueAdminClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, queue.QueueAdminClient{}, ErrorResult(err.Error())
	}
	c, err := queue.NewQueueAdminClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, queue.QueueAdminClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// DataPlaneClientForQueue builds the data-plane client for one queue. The data plane lives on the
// queue's OWN messagesEndpoint (a cell-specific host, not the region), so we resolve it from the
// queue OCID via the admin client's GetQueue and point the QueueClient's Host at it. Keeps every
// data-plane action operator-simple: the user supplies the queue, never a raw endpoint.
func DataPlaneClientForQueue(inputs []*coreflow.Connection, queueID string) (auth Auth, client queue.QueueClient, errResult map[string]interface{}) {
	a, admin, errRes := AdminClient(inputs)
	if errRes != nil {
		return Auth{}, queue.QueueClient{}, errRes
	}
	if strings.TrimSpace(queueID) == "" {
		return Auth{}, queue.QueueClient{}, ErrorResult("queue OCID is required")
	}
	resp, err := admin.GetQueue(Context(), queue.GetQueueRequest{QueueId: &queueID})
	if err != nil {
		return Auth{}, queue.QueueClient{}, ErrorResult(a.OCIError(err))
	}
	endpoint := Str(resp.MessagesEndpoint)
	if endpoint == "" {
		return Auth{}, queue.QueueClient{}, ErrorResult(fmt.Sprintf("queue %q has no messages endpoint yet (it is %s) — wait until it is ACTIVE", Str(resp.DisplayName), string(resp.LifecycleState)))
	}
	c, err := queue.NewQueueClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, queue.QueueClient{}, ErrorResult(a.OCIError(err))
	}
	c.Host = endpoint
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

func SummariseQueue(q *queue.Queue) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(q.Id),
		"display_name":       Str(q.DisplayName),
		"compartment_id":     Str(q.CompartmentId),
		"lifecycle_state":    string(q.LifecycleState),
		"messages_endpoint":  Str(q.MessagesEndpoint),
		"retention_seconds":  IntOrNil(q.RetentionInSeconds),
		"visibility_seconds": IntOrNil(q.VisibilityInSeconds),
		"timeout_seconds":    IntOrNil(q.TimeoutInSeconds),
		"dlq_delivery_count": IntOrNil(q.DeadLetterQueueDeliveryCount),
		"time_created":       FormatTime(q.TimeCreated),
	}
}

func SummariseQueueSummary(q *queue.QueueSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                Str(q.Id),
		"display_name":      Str(q.DisplayName),
		"compartment_id":    Str(q.CompartmentId),
		"lifecycle_state":   string(q.LifecycleState),
		"messages_endpoint": Str(q.MessagesEndpoint),
		"time_created":      FormatTime(q.TimeCreated),
	}
}

// SummariseMessage shapes one consumed queue message. Content is plain text on the wire.
func SummariseMessage(m *queue.GetMessage) map[string]interface{} {
	return map[string]interface{}{
		"id":             Int64OrNil(m.Id),
		"content":        Str(m.Content),
		"receipt":        Str(m.Receipt),
		"delivery_count": IntOrNil(m.DeliveryCount),
		"visible_after":  FormatTime(m.VisibleAfter),
		"expire_after":   FormatTime(m.ExpireAfter),
		"created_at":     FormatTime(m.CreatedAt),
	}
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

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
