// Package notifications holds what every Oracle Cloud (OCI) Notifications (ONS) action
// shares: the API-signing-key credential, the two regional clients this service spans, the
// resource summarisers, and the input/result helpers. Like the sibling OCI packages it has
// no Execute function, so the manifest generator skips it — but its category.go supplies
// the "Notifications" sub-group.
//
// ONS has two planes, both regional (no per-resource endpoints):
//   - NotificationControlPlaneClient — topics (create/get/list/update/delete/move).
//   - NotificationDataPlaneClient    — subscriptions (create/get/list/update/delete/confirm/
//     unsubscribe/resend/move) and publishing messages to a topic.
package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/ons"

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
// Client preambles (two regional planes)
// ---------------------------------------------------------------------------

// ControlPlaneClient is the regional topics client.
func ControlPlaneClient(inputs []*coreflow.Connection) (auth Auth, client ons.NotificationControlPlaneClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, ons.NotificationControlPlaneClient{}, ErrorResult(err.Error())
	}
	c, err := ons.NewNotificationControlPlaneClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, ons.NotificationControlPlaneClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// DataPlaneClient is the regional subscriptions + publish client.
func DataPlaneClient(inputs []*coreflow.Connection) (auth Auth, client ons.NotificationDataPlaneClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, ons.NotificationDataPlaneClient{}, ErrorResult(err.Error())
	}
	c, err := ons.NewNotificationDataPlaneClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, ons.NotificationDataPlaneClient{}, ErrorResult(a.OCIError(err))
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

// FormatEpochMillis renders an ONS *int64 created-time (epoch milliseconds) as RFC3339.
func FormatEpochMillis(p *int64) string {
	if p == nil {
		return ""
	}
	return time.UnixMilli(*p).UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// Resource summarisers
// ---------------------------------------------------------------------------

func SummariseTopic(t *ons.NotificationTopic) map[string]interface{} {
	return map[string]interface{}{
		"topic_id":        Str(t.TopicId),
		"name":            Str(t.Name),
		"description":     Str(t.Description),
		"compartment_id":  Str(t.CompartmentId),
		"lifecycle_state": string(t.LifecycleState),
		"api_endpoint":    Str(t.ApiEndpoint),
		"short_topic_id":  Str(t.ShortTopicId),
		"time_created":    FormatTime(t.TimeCreated),
	}
}

func SummariseTopicSummary(t *ons.NotificationTopicSummary) map[string]interface{} {
	return map[string]interface{}{
		"topic_id":        Str(t.TopicId),
		"name":            Str(t.Name),
		"description":     Str(t.Description),
		"compartment_id":  Str(t.CompartmentId),
		"lifecycle_state": string(t.LifecycleState),
		"api_endpoint":    Str(t.ApiEndpoint),
		"short_topic_id":  Str(t.ShortTopicId),
		"time_created":    FormatTime(t.TimeCreated),
	}
}

func SummariseSubscription(s *ons.Subscription) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(s.Id),
		"topic_id":        Str(s.TopicId),
		"protocol":        Str(s.Protocol),
		"endpoint":        Str(s.Endpoint),
		"compartment_id":  Str(s.CompartmentId),
		"lifecycle_state": string(s.LifecycleState),
		"deliver_policy":  Str(s.DeliverPolicy),
		"created_time":    FormatEpochMillis(s.CreatedTime),
	}
}

func SummariseSubscriptionSummary(s *ons.SubscriptionSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(s.Id),
		"topic_id":        Str(s.TopicId),
		"protocol":        Str(s.Protocol),
		"endpoint":        Str(s.Endpoint),
		"compartment_id":  Str(s.CompartmentId),
		"lifecycle_state": string(s.LifecycleState),
		"created_time":    FormatEpochMillis(s.CreatedTime),
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
