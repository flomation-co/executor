// Package datacatalog holds what every Oracle Cloud (OCI) Data Catalog action shares: the
// API-signing-key credential, the regional DataCatalogClient, the resource summarisers, and the
// input/result helpers. Like the sibling OCI packages it has no Execute function, so the manifest
// generator skips it — but its category.go supplies the "Data Catalog" sub-group.
//
// OCI Data Catalog is a single regional service. Note the resource shape: the catalog itself is an
// OCID-identified regional resource (Id, LifecycleState), but every CHILD resource — data assets,
// connections, glossaries, terms, entities — is identified by an immutable string Key and is scoped
// by CatalogId, not an OCID. Summarisers use Str() for those Key fields.
package datacatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/datacatalog"

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

// Client is the regional Data Catalog client.
func Client(inputs []*coreflow.Connection) (auth Auth, client datacatalog.DataCatalogClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, datacatalog.DataCatalogClient{}, ErrorResult(err.Error())
	}
	c, err := datacatalog.NewDataCatalogClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, datacatalog.DataCatalogClient{}, ErrorResult(a.OCIError(err))
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

func FormatTime(t *common.SDKTime) string {
	if t == nil {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// Resource summarisers
// ---------------------------------------------------------------------------

// SummariseCatalog shapes a data catalog instance — the OCID-identified regional resource that
// every other Data Catalog resource is scoped under.
func SummariseCatalog(r *datacatalog.Catalog) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(r.Id),
		"display_name":        Str(r.DisplayName),
		"compartment_id":      Str(r.CompartmentId),
		"lifecycle_state":     string(r.LifecycleState),
		"number_of_objects":   IntOrNil(r.NumberOfObjects),
		"service_api_url":     Str(r.ServiceApiUrl),
		"service_console_url": Str(r.ServiceConsoleUrl),
		"time_created":        FormatTime(r.TimeCreated),
	}
}

func SummariseCatalogSummary(r *datacatalog.CatalogSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                Str(r.Id),
		"display_name":      Str(r.DisplayName),
		"compartment_id":    Str(r.CompartmentId),
		"lifecycle_state":   string(r.LifecycleState),
		"number_of_objects": IntOrNil(r.NumberOfObjects),
		"time_created":      FormatTime(r.TimeCreated),
	}
}

// SummariseDataAsset shapes a data asset — a physical store or stream of data. Identified by a
// string Key, scoped by CatalogId.
func SummariseDataAsset(r *datacatalog.DataAsset) map[string]interface{} {
	return map[string]interface{}{
		"key":             Str(r.Key),
		"display_name":    Str(r.DisplayName),
		"description":     Str(r.Description),
		"catalog_id":      Str(r.CatalogId),
		"type_key":        Str(r.TypeKey),
		"external_key":    Str(r.ExternalKey),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
		"time_harvested":  FormatTime(r.TimeHarvested),
	}
}

func SummariseDataAssetSummary(r *datacatalog.DataAssetSummary) map[string]interface{} {
	return map[string]interface{}{
		"key":             Str(r.Key),
		"display_name":    Str(r.DisplayName),
		"description":     Str(r.Description),
		"catalog_id":      Str(r.CatalogId),
		"type_key":        Str(r.TypeKey),
		"external_key":    Str(r.ExternalKey),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

// SummariseConnection shapes a connection to a data asset. Identified by a string Key, scoped by
// its parent DataAssetKey.
func SummariseConnection(r *datacatalog.Connection) map[string]interface{} {
	return map[string]interface{}{
		"key":             Str(r.Key),
		"display_name":    Str(r.DisplayName),
		"description":     Str(r.Description),
		"data_asset_key":  Str(r.DataAssetKey),
		"type_key":        Str(r.TypeKey),
		"is_default":      Bool(r.IsDefault),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

func SummariseConnectionSummary(r *datacatalog.ConnectionSummary) map[string]interface{} {
	return map[string]interface{}{
		"key":             Str(r.Key),
		"display_name":    Str(r.DisplayName),
		"description":     Str(r.Description),
		"data_asset_key":  Str(r.DataAssetKey),
		"type_key":        Str(r.TypeKey),
		"is_default":      Bool(r.IsDefault),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

// SummariseGlossary shapes a business glossary. Identified by a string Key, scoped by CatalogId.
func SummariseGlossary(r *datacatalog.Glossary) map[string]interface{} {
	return map[string]interface{}{
		"key":             Str(r.Key),
		"display_name":    Str(r.DisplayName),
		"description":     Str(r.Description),
		"catalog_id":      Str(r.CatalogId),
		"owner":           Str(r.Owner),
		"workflow_status": string(r.WorkflowStatus),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

func SummariseGlossarySummary(r *datacatalog.GlossarySummary) map[string]interface{} {
	return map[string]interface{}{
		"key":             Str(r.Key),
		"display_name":    Str(r.DisplayName),
		"description":     Str(r.Description),
		"catalog_id":      Str(r.CatalogId),
		"workflow_status": string(r.WorkflowStatus),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

// SummariseTerm shapes a business glossary term. Identified by a string Key, scoped by GlossaryKey.
func SummariseTerm(r *datacatalog.Term) map[string]interface{} {
	return map[string]interface{}{
		"key":                     Str(r.Key),
		"display_name":            Str(r.DisplayName),
		"description":             Str(r.Description),
		"glossary_key":            Str(r.GlossaryKey),
		"parent_term_key":         Str(r.ParentTermKey),
		"path":                    Str(r.Path),
		"workflow_status":         string(r.WorkflowStatus),
		"lifecycle_state":         string(r.LifecycleState),
		"associated_object_count": IntOrNil(r.AssociatedObjectCount),
		"time_created":            FormatTime(r.TimeCreated),
	}
}

func SummariseTermSummary(r *datacatalog.TermSummary) map[string]interface{} {
	return map[string]interface{}{
		"key":                     Str(r.Key),
		"display_name":            Str(r.DisplayName),
		"description":             Str(r.Description),
		"glossary_key":            Str(r.GlossaryKey),
		"parent_term_key":         Str(r.ParentTermKey),
		"path":                    Str(r.Path),
		"workflow_status":         string(r.WorkflowStatus),
		"lifecycle_state":         string(r.LifecycleState),
		"associated_object_count": IntOrNil(r.AssociatedObjectCount),
		"time_created":            FormatTime(r.TimeCreated),
	}
}

// SummariseEntity shapes a data entity (a harvested table, view, file or stream). Identified by a
// string Key, scoped by DataAssetKey.
func SummariseEntity(r *datacatalog.Entity) map[string]interface{} {
	return map[string]interface{}{
		"key":             Str(r.Key),
		"display_name":    Str(r.DisplayName),
		"business_name":   Str(r.BusinessName),
		"description":     Str(r.Description),
		"data_asset_key":  Str(r.DataAssetKey),
		"folder_key":      Str(r.FolderKey),
		"type_key":        Str(r.TypeKey),
		"is_logical":      Bool(r.IsLogical),
		"path":            Str(r.Path),
		"harvest_status":  string(r.HarvestStatus),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

func SummariseEntitySummary(r *datacatalog.EntitySummary) map[string]interface{} {
	return map[string]interface{}{
		"key":             Str(r.Key),
		"display_name":    Str(r.DisplayName),
		"business_name":   Str(r.BusinessName),
		"description":     Str(r.Description),
		"data_asset_key":  Str(r.DataAssetKey),
		"folder_key":      Str(r.FolderKey),
		"type_key":        Str(r.TypeKey),
		"is_logical":      Bool(r.IsLogical),
		"path":            Str(r.Path),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
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
