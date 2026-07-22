// Package mysql holds what every Oracle Cloud (OCI) MySQL HeatWave action shares: the
// API-signing-key credential, the four service clients this node spans, the resource
// summarisers, and the input/result helpers. Like the sibling OCI packages it has no Execute
// function, so the manifest generator skips it — but its category.go supplies the
// "MySQL HeatWave" sub-group.
//
// OCI MySQL HeatWave is one regional service exposed through several SDK clients:
//   - DbSystemClient   — DB systems (create/get/list/update/delete, start/stop/restart) AND every
//     HeatWave-cluster operation (add/get/update/delete, start/stop/restart). There is no separate
//     HeatWave client in the SDK; HeatWaveClient below is a semantically-named alias that returns
//     the same DbSystemClient.
//   - DbBackupsClient  — backups (create/get/list/update/delete, copy/export/validate).
//   - MysqlaasClient   — configurations (create/get/list/update/delete), shapes, versions.
package mysql

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/mysql"

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

// DbSystemClient is the regional client for DB systems and HeatWave clusters.
func DbSystemClient(inputs []*coreflow.Connection) (auth Auth, client mysql.DbSystemClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, mysql.DbSystemClient{}, ErrorResult(err.Error())
	}
	c, err := mysql.NewDbSystemClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, mysql.DbSystemClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// BackupsClient is the regional client for MySQL backups.
func BackupsClient(inputs []*coreflow.Connection) (auth Auth, client mysql.DbBackupsClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, mysql.DbBackupsClient{}, ErrorResult(err.Error())
	}
	c, err := mysql.NewDbBackupsClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, mysql.DbBackupsClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// ConfigClient is the regional MySQLaaS client for configurations, shapes and versions.
func ConfigClient(inputs []*coreflow.Connection) (auth Auth, client mysql.MysqlaasClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, mysql.MysqlaasClient{}, ErrorResult(err.Error())
	}
	c, err := mysql.NewMysqlaasClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, mysql.MysqlaasClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// HeatWaveClient returns the DbSystemClient under a HeatWave-specific name: the SDK hosts every
// HeatWave-cluster operation (add/get/update/delete, start/stop/restart) on DbSystemClient, so this
// is the same client with a clearer call-site name for the HeatWave actions.
func HeatWaveClient(inputs []*coreflow.Connection) (auth Auth, client mysql.DbSystemClient, errResult map[string]interface{}) {
	return DbSystemClient(inputs)
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

// summariseEndpoints flattens the per-DB-system network endpoints into a compact list.
func summariseEndpoints(eps []mysql.DbSystemEndpoint) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(eps))
	for i := range eps {
		e := eps[i]
		out = append(out, map[string]interface{}{
			"hostname":   Str(e.Hostname),
			"ip_address": Str(e.IpAddress),
			"port":       IntOrNil(e.Port),
			"port_x":     IntOrNil(e.PortX),
			"status":     string(e.Status),
		})
	}
	return out
}

func SummariseDbSystem(r *mysql.DbSystem) map[string]interface{} {
	return map[string]interface{}{
		"id":                            Str(r.Id),
		"display_name":                  Str(r.DisplayName),
		"compartment_id":                Str(r.CompartmentId),
		"mysql_version":                 Str(r.MysqlVersion),
		"shape_name":                    Str(r.ShapeName),
		"lifecycle_state":               string(r.LifecycleState),
		"is_highly_available":           Bool(r.IsHighlyAvailable),
		"is_heat_wave_cluster_attached": Bool(r.IsHeatWaveClusterAttached),
		"endpoints":                     summariseEndpoints(r.Endpoints),
		"time_created":                  FormatTime(r.TimeCreated),
	}
}

func SummariseDbSystemSummary(r *mysql.DbSystemSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                            Str(r.Id),
		"display_name":                  Str(r.DisplayName),
		"compartment_id":                Str(r.CompartmentId),
		"mysql_version":                 Str(r.MysqlVersion),
		"shape_name":                    Str(r.ShapeName),
		"lifecycle_state":               string(r.LifecycleState),
		"is_highly_available":           Bool(r.IsHighlyAvailable),
		"is_heat_wave_cluster_attached": Bool(r.IsHeatWaveClusterAttached),
		"endpoints":                     summariseEndpoints(r.Endpoints),
		"time_created":                  FormatTime(r.TimeCreated),
	}
}

func SummariseBackup(r *mysql.Backup) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(r.Id),
		"display_name":       Str(r.DisplayName),
		"compartment_id":     Str(r.CompartmentId),
		"backup_type":        string(r.BackupType),
		"creation_type":      string(r.CreationType),
		"lifecycle_state":    string(r.LifecycleState),
		"db_system_id":       Str(r.DbSystemId),
		"backup_size_in_gbs": IntOrNil(r.BackupSizeInGBs),
		"mysql_version":      Str(r.MysqlVersion),
		"time_created":       FormatTime(r.TimeCreated),
	}
}

func SummariseBackupSummary(r *mysql.BackupSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                 Str(r.Id),
		"display_name":       Str(r.DisplayName),
		"compartment_id":     Str(r.CompartmentId),
		"backup_type":        string(r.BackupType),
		"creation_type":      string(r.CreationType),
		"lifecycle_state":    string(r.LifecycleState),
		"db_system_id":       Str(r.DbSystemId),
		"backup_size_in_gbs": IntOrNil(r.BackupSizeInGBs),
		"mysql_version":      Str(r.MysqlVersion),
		"time_created":       FormatTime(r.TimeCreated),
	}
}

func SummariseConfiguration(r *mysql.Configuration) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(r.Id),
		"display_name":    Str(r.DisplayName),
		"description":     Str(r.Description),
		"compartment_id":  Str(r.CompartmentId),
		"shape_name":      Str(r.ShapeName),
		"type":            string(r.Type),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

func SummariseConfigurationSummary(r *mysql.ConfigurationSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(r.Id),
		"display_name":    Str(r.DisplayName),
		"description":     Str(r.Description),
		"compartment_id":  Str(r.CompartmentId),
		"shape_name":      Str(r.ShapeName),
		"type":            string(r.Type),
		"lifecycle_state": string(r.LifecycleState),
		"time_created":    FormatTime(r.TimeCreated),
	}
}

func SummariseHeatWaveCluster(r *mysql.HeatWaveCluster) map[string]interface{} {
	return map[string]interface{}{
		"db_system_id":         Str(r.DbSystemId),
		"shape_name":           Str(r.ShapeName),
		"cluster_size":         IntOrNil(r.ClusterSize),
		"node_count":           len(r.ClusterNodes),
		"lifecycle_state":      string(r.LifecycleState),
		"is_lakehouse_enabled": Bool(r.IsLakehouseEnabled),
		"time_created":         FormatTime(r.TimeCreated),
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
