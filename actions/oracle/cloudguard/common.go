// Package cloudguard holds what every Oracle Cloud Guard action shares: the API-signing-key
// credential, the regional CloudGuardClient, the resource summarisers, and the input/result
// helpers. Like the sibling OCI packages it has no Execute function, so the manifest generator
// skips it — but its category.go supplies the "Cloud Guard" sub-group.
//
// Cloud Guard monitors a tenancy's security posture: detector recipes classify risky activity
// and configuration, responder recipes act on it, targets scope the monitoring to compartments,
// managed lists parameterise the rules, and problems are the findings Cloud Guard surfaces.
package cloudguard

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
	"github.com/oracle/oci-go-sdk/v65/common"

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

// Client is the regional Cloud Guard client.
func Client(inputs []*coreflow.Connection) (auth Auth, client cloudguard.CloudGuardClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, cloudguard.CloudGuardClient{}, ErrorResult(err.Error())
	}
	c, err := cloudguard.NewCloudGuardClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, cloudguard.CloudGuardClient{}, ErrorResult(a.OCIError(err))
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

// OptionalBoolPtr returns nil when the field is blank so a partial update leaves it unchanged.
func OptionalBoolPtr(name string, inputs []*coreflow.Connection) *bool {
	c := coreflow.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	if b := c.Boolean(); b != nil {
		return b
	}
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil
	}
	v := strings.EqualFold(raw, "true")
	return &v
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

func Bool(p *bool) interface{} {
	if p == nil {
		return nil
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

func FloatOrNil(p *float64) interface{} {
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

func SummariseDetectorRecipe(r *cloudguard.DetectorRecipe) map[string]interface{} {
	return map[string]interface{}{
		"id":                        Str(r.Id),
		"display_name":              Str(r.DisplayName),
		"description":               Str(r.Description),
		"compartment_id":            Str(r.CompartmentId),
		"owner":                     string(r.Owner),
		"detector":                  string(r.Detector),
		"detector_recipe_type":      string(r.DetectorRecipeType),
		"source_detector_recipe_id": Str(r.SourceDetectorRecipeId),
		"source_data_retention":     IntOrNil(r.SourceDataRetention),
		"target_count":              len(r.TargetIds),
		"lifecycle_state":           string(r.LifecycleState),
		"time_created":              FormatTime(r.TimeCreated),
		"time_updated":              FormatTime(r.TimeUpdated),
	}
}

func SummariseDetectorRecipeSummary(r *cloudguard.DetectorRecipeSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                        Str(r.Id),
		"display_name":              Str(r.DisplayName),
		"description":               Str(r.Description),
		"compartment_id":            Str(r.CompartmentId),
		"owner":                     string(r.Owner),
		"detector":                  string(r.Detector),
		"detector_recipe_type":      string(r.DetectorRecipeType),
		"source_detector_recipe_id": Str(r.SourceDetectorRecipeId),
		"source_data_retention":     IntOrNil(r.SourceDataRetention),
		"lifecycle_state":           string(r.LifecycleState),
		"time_created":              FormatTime(r.TimeCreated),
		"time_updated":              FormatTime(r.TimeUpdated),
	}
}

func SummariseTarget(t *cloudguard.Target) map[string]interface{} {
	return map[string]interface{}{
		"id":                   Str(t.Id),
		"display_name":         Str(t.DisplayName),
		"description":          Str(t.Description),
		"compartment_id":       Str(t.CompartmentId),
		"target_resource_type": string(t.TargetResourceType),
		"target_resource_id":   Str(t.TargetResourceId),
		"recipe_count":         IntOrNil(t.RecipeCount),
		"lifecycle_state":      string(t.LifecycleState),
		"time_created":         FormatTime(t.TimeCreated),
		"time_updated":         FormatTime(t.TimeUpdated),
	}
}

func SummariseTargetSummary(t *cloudguard.TargetSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                   Str(t.Id),
		"display_name":         Str(t.DisplayName),
		"compartment_id":       Str(t.CompartmentId),
		"target_resource_type": string(t.TargetResourceType),
		"target_resource_id":   Str(t.TargetResourceId),
		"recipe_count":         IntOrNil(t.RecipeCount),
		"lifecycle_state":      string(t.LifecycleState),
		"time_created":         FormatTime(t.TimeCreated),
		"time_updated":         FormatTime(t.TimeUpdated),
	}
}

func SummariseProblem(p *cloudguard.Problem) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(p.Id),
		"compartment_id":      Str(p.CompartmentId),
		"detector_rule_id":    Str(p.DetectorRuleId),
		"detector_id":         string(p.DetectorId),
		"risk_level":          string(p.RiskLevel),
		"risk_score":          FloatOrNil(p.RiskScore),
		"resource_id":         Str(p.ResourceId),
		"resource_name":       Str(p.ResourceName),
		"resource_type":       Str(p.ResourceType),
		"target_id":           Str(p.TargetId),
		"lifecycle_state":     string(p.LifecycleState),
		"lifecycle_detail":    string(p.LifecycleDetail),
		"time_first_detected": FormatTime(p.TimeFirstDetected),
		"time_last_detected":  FormatTime(p.TimeLastDetected),
	}
}

func SummariseProblemSummary(p *cloudguard.ProblemSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(p.Id),
		"compartment_id":      Str(p.CompartmentId),
		"detector_rule_id":    Str(p.DetectorRuleId),
		"detector_id":         string(p.DetectorId),
		"risk_level":          string(p.RiskLevel),
		"risk_score":          FloatOrNil(p.RiskScore),
		"resource_id":         Str(p.ResourceId),
		"resource_name":       Str(p.ResourceName),
		"resource_type":       Str(p.ResourceType),
		"target_id":           Str(p.TargetId),
		"lifecycle_state":     string(p.LifecycleState),
		"lifecycle_detail":    string(p.LifecycleDetail),
		"time_first_detected": FormatTime(p.TimeFirstDetected),
		"time_last_detected":  FormatTime(p.TimeLastDetected),
	}
}

func SummariseManagedList(m *cloudguard.ManagedList) map[string]interface{} {
	return map[string]interface{}{
		"id":                     Str(m.Id),
		"display_name":           Str(m.DisplayName),
		"description":            Str(m.Description),
		"compartment_id":         Str(m.CompartmentId),
		"list_type":              string(m.ListType),
		"feed_provider":          string(m.FeedProvider),
		"group":                  Str(m.Group),
		"is_editable":            Bool(m.IsEditable),
		"source_managed_list_id": Str(m.SourceManagedListId),
		"list_item_count":        len(m.ListItems),
		"lifecycle_state":        string(m.LifecycleState),
		"time_created":           FormatTime(m.TimeCreated),
		"time_updated":           FormatTime(m.TimeUpdated),
	}
}

func SummariseManagedListSummary(m *cloudguard.ManagedListSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                     Str(m.Id),
		"display_name":           Str(m.DisplayName),
		"description":            Str(m.Description),
		"compartment_id":         Str(m.CompartmentId),
		"list_type":              string(m.ListType),
		"feed_provider":          string(m.FeedProvider),
		"group":                  Str(m.Group),
		"is_editable":            Bool(m.IsEditable),
		"source_managed_list_id": Str(m.SourceManagedListId),
		"list_item_count":        len(m.ListItems),
		"lifecycle_state":        string(m.LifecycleState),
		"time_created":           FormatTime(m.TimeCreated),
		"time_updated":           FormatTime(m.TimeUpdated),
	}
}

func SummariseResponderRecipe(r *cloudguard.ResponderRecipe) map[string]interface{} {
	return map[string]interface{}{
		"id":                         Str(r.Id),
		"display_name":               Str(r.DisplayName),
		"description":                Str(r.Description),
		"compartment_id":             Str(r.CompartmentId),
		"owner":                      string(r.Owner),
		"source_responder_recipe_id": Str(r.SourceResponderRecipeId),
		"lifecycle_state":            string(r.LifecycleState),
		"time_created":               FormatTime(r.TimeCreated),
		"time_updated":               FormatTime(r.TimeUpdated),
	}
}

func SummariseResponderRecipeSummary(r *cloudguard.ResponderRecipeSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                         Str(r.Id),
		"display_name":               Str(r.DisplayName),
		"description":                Str(r.Description),
		"compartment_id":             Str(r.CompartmentId),
		"owner":                      string(r.Owner),
		"source_responder_recipe_id": Str(r.SourceResponderRecipeId),
		"lifecycle_state":            string(r.LifecycleState),
		"time_created":               FormatTime(r.TimeCreated),
		"time_updated":               FormatTime(r.TimeUpdated),
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
