// Package vault holds what every Oracle Cloud (OCI) Vault / KMS action shares: the
// API-signing-key credential, the FIVE clients this service spans, the client preambles,
// the resource summarisers, and the input/result helpers. Like the sibling OCI packages
// it has no Execute function, so the manifest generator skips it — but its category.go
// supplies the "Vault" sub-group.
//
// The defining KMS quirk: key-management and crypto operations do NOT go to the regional
// endpoint — each vault has its OWN management and crypto endpoints. So those actions
// take a vault OCID, fetch the vault (KmsVaultClient.GetVault) to read its
// ManagementEndpoint / CryptoEndpoint, and build a KmsManagementClient / KmsCryptoClient
// pointed at that endpoint. Vaults themselves and secrets use regional clients
// (KmsVaultClient, vault.VaultsClient, secrets.SecretsClient). Endpoints come from OCI's
// own GetVault response, so they are trusted (no SSRF surface beyond the signed API).
package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"github.com/oracle/oci-go-sdk/v65/secrets"
	ovault "github.com/oracle/oci-go-sdk/v65/vault"

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
// Client preambles (five clients — see the package doc)
// ---------------------------------------------------------------------------

// VaultClient is the regional KMS vault client (vault CRUD + backup/restore/replicas).
func VaultClient(inputs []*coreflow.Connection) (auth Auth, client keymanagement.KmsVaultClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, keymanagement.KmsVaultClient{}, ErrorResult(err.Error())
	}
	c, err := keymanagement.NewKmsVaultClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, keymanagement.KmsVaultClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// getVaultEndpoints fetches a vault and returns its management + crypto endpoints. The
// KMS management/crypto clients are vault-specific, so every key/crypto action resolves
// these first.
func (a Auth) getVaultEndpoints(vaultID string) (managementEndpoint, cryptoEndpoint string, err error) {
	vc, err := keymanagement.NewKmsVaultClientWithConfigurationProvider(a.provider)
	if err != nil {
		return "", "", err
	}
	resp, err := vc.GetVault(context.Background(), keymanagement.GetVaultRequest{VaultId: &vaultID})
	if err != nil {
		return "", "", err
	}
	return Str(resp.Vault.ManagementEndpoint), Str(resp.Vault.CryptoEndpoint), nil
}

// ManagementForVault reads a vault OCID (named by inputName), resolves the vault's
// management endpoint, and returns a KmsManagementClient pointed at it (for key + key-
// version operations).
func ManagementForVault(inputs []*coreflow.Connection, inputName string) (auth Auth, client keymanagement.KmsManagementClient, vaultID string, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, keymanagement.KmsManagementClient{}, "", ErrorResult(err.Error())
	}
	vid, err := RequiredString(inputName, inputs)
	if err != nil {
		return Auth{}, keymanagement.KmsManagementClient{}, "", ErrorResult(err.Error())
	}
	mgmtEP, _, err := a.getVaultEndpoints(vid)
	if err != nil {
		return Auth{}, keymanagement.KmsManagementClient{}, "", ErrorResult(a.OCIError(err))
	}
	if mgmtEP == "" {
		return Auth{}, keymanagement.KmsManagementClient{}, "", ErrorResult("the vault has no management endpoint yet — it may still be provisioning (the endpoint DNS can lag a few minutes behind the vault becoming ACTIVE); try again shortly")
	}
	c, err := keymanagement.NewKmsManagementClientWithConfigurationProvider(a.provider, mgmtEP)
	if err != nil {
		return Auth{}, keymanagement.KmsManagementClient{}, "", ErrorResult(a.OCIError(err))
	}
	return a, c, vid, nil
}

// CryptoForVault is ManagementForVault's sibling for the crypto client
// (encrypt/decrypt/sign/verify/generate-DEK/export).
func CryptoForVault(inputs []*coreflow.Connection, inputName string) (auth Auth, client keymanagement.KmsCryptoClient, vaultID string, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, keymanagement.KmsCryptoClient{}, "", ErrorResult(err.Error())
	}
	vid, err := RequiredString(inputName, inputs)
	if err != nil {
		return Auth{}, keymanagement.KmsCryptoClient{}, "", ErrorResult(err.Error())
	}
	_, cryptoEP, err := a.getVaultEndpoints(vid)
	if err != nil {
		return Auth{}, keymanagement.KmsCryptoClient{}, "", ErrorResult(a.OCIError(err))
	}
	if cryptoEP == "" {
		return Auth{}, keymanagement.KmsCryptoClient{}, "", ErrorResult("the vault has no crypto endpoint yet — it may still be provisioning (the endpoint DNS can lag a few minutes behind the vault becoming ACTIVE); try again shortly")
	}
	c, err := keymanagement.NewKmsCryptoClientWithConfigurationProvider(a.provider, cryptoEP)
	if err != nil {
		return Auth{}, keymanagement.KmsCryptoClient{}, "", ErrorResult(a.OCIError(err))
	}
	return a, c, vid, nil
}

// SecretsMgmtClient is the regional secrets-management client (secret CRUD + rotation +
// versions). Secrets live in a vault but are managed regionally, not via the vault endpoint.
func SecretsMgmtClient(inputs []*coreflow.Connection) (auth Auth, client ovault.VaultsClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, ovault.VaultsClient{}, ErrorResult(err.Error())
	}
	c, err := ovault.NewVaultsClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, ovault.VaultsClient{}, ErrorResult(a.OCIError(err))
	}
	return a, c, nil
}

// SecretRetrievalClient is the regional secrets-retrieval client (get the actual secret
// value — the "bundle").
func SecretRetrievalClient(inputs []*coreflow.Connection) (auth Auth, client secrets.SecretsClient, errResult map[string]interface{}) {
	a, err := GetAuth(inputs)
	if err != nil {
		return Auth{}, secrets.SecretsClient{}, ErrorResult(err.Error())
	}
	c, err := secrets.NewSecretsClientWithConfigurationProvider(a.provider)
	if err != nil {
		return Auth{}, secrets.SecretsClient{}, ErrorResult(a.OCIError(err))
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

// RequiredInt64 reads a mandatory whole-number input (e.g. a secret version number).
func RequiredInt64(name string, inputs []*coreflow.Connection) (int64, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", fieldLabel(name))
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a whole number", fieldLabel(name))
	}
	return n, nil
}

// OptionalInt64 reads an optional whole-number input; ok is false when it was left blank.
func OptionalInt64(name string, inputs []*coreflow.Connection) (int64, bool, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return 0, false, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be a whole number", fieldLabel(name))
	}
	return n, true, nil
}

// ParseSDKTime turns an optional RFC3339 timestamp input into an *common.SDKTime. When the
// input is blank it returns (nil, false, nil) — callers omit the field so OCI applies its
// own default (e.g. the earliest permissible deletion time).
func ParseSDKTime(name string, inputs []*coreflow.Connection) (*common.SDKTime, bool, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, false, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, false, fmt.Errorf("%s must be an RFC3339 timestamp, e.g. 2026-08-01T00:00:00Z", fieldLabel(name))
	}
	sdk := common.SDKTime{Time: t}
	return &sdk, true, nil
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

// ---------------------------------------------------------------------------
// Resource summarisers
// ---------------------------------------------------------------------------

func SummariseVault(v *keymanagement.Vault) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(v.Id),
		"display_name":        Str(v.DisplayName),
		"compartment_id":      Str(v.CompartmentId),
		"vault_type":          string(v.VaultType),
		"lifecycle_state":     string(v.LifecycleState),
		"management_endpoint": Str(v.ManagementEndpoint),
		"crypto_endpoint":     Str(v.CryptoEndpoint),
		"wrapping_key_id":     Str(v.WrappingkeyId),
		"is_primary":          v.IsPrimary != nil && *v.IsPrimary,
		"time_created":        FormatTime(v.TimeCreated),
	}
}

func SummariseVaultSummary(v *keymanagement.VaultSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(v.Id),
		"display_name":        Str(v.DisplayName),
		"compartment_id":      Str(v.CompartmentId),
		"vault_type":          string(v.VaultType),
		"lifecycle_state":     string(v.LifecycleState),
		"management_endpoint": Str(v.ManagementEndpoint),
		"crypto_endpoint":     Str(v.CryptoEndpoint),
		"time_created":        FormatTime(v.TimeCreated),
	}
}

func keyShape(s *keymanagement.KeyShape) map[string]interface{} {
	if s == nil {
		return nil
	}
	m := map[string]interface{}{"algorithm": string(s.Algorithm)}
	if s.Length != nil {
		m["length"] = *s.Length
	}
	if s.CurveId != "" {
		m["curve_id"] = string(s.CurveId)
	}
	return m
}

func SummariseKey(k *keymanagement.Key) map[string]interface{} {
	return map[string]interface{}{
		"id":                  Str(k.Id),
		"display_name":        Str(k.DisplayName),
		"compartment_id":      Str(k.CompartmentId),
		"vault_id":            Str(k.VaultId),
		"current_key_version": Str(k.CurrentKeyVersion),
		"protection_mode":     string(k.ProtectionMode),
		"lifecycle_state":     string(k.LifecycleState),
		"key_shape":           keyShape(k.KeyShape),
		"time_created":        FormatTime(k.TimeCreated),
	}
}

func SummariseKeySummary(k *keymanagement.KeySummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(k.Id),
		"display_name":    Str(k.DisplayName),
		"compartment_id":  Str(k.CompartmentId),
		"vault_id":        Str(k.VaultId),
		"protection_mode": string(k.ProtectionMode),
		"lifecycle_state": string(k.LifecycleState),
		"time_created":    FormatTime(k.TimeCreated),
	}
}

func SummariseKeyVersion(v *keymanagement.KeyVersion) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(v.Id),
		"key_id":          Str(v.KeyId),
		"vault_id":        Str(v.VaultId),
		"compartment_id":  Str(v.CompartmentId),
		"lifecycle_state": string(v.LifecycleState),
		"time_created":    FormatTime(v.TimeCreated),
	}
}

func SummariseKeyVersionSummary(v *keymanagement.KeyVersionSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(v.Id),
		"key_id":          Str(v.KeyId),
		"vault_id":        Str(v.VaultId),
		"compartment_id":  Str(v.CompartmentId),
		"origin":          string(v.Origin),
		"lifecycle_state": string(v.LifecycleState),
		"time_created":    FormatTime(v.TimeCreated),
	}
}

func SummariseVaultReplica(r *keymanagement.VaultReplicaSummary) map[string]interface{} {
	return map[string]interface{}{
		"region":              Str(r.Region),
		"status":              string(r.Status),
		"management_endpoint": Str(r.ManagementEndpoint),
		"crypto_endpoint":     Str(r.CryptoEndpoint),
	}
}

func SummariseSecretVersion(v *ovault.SecretVersion) map[string]interface{} {
	stages := make([]string, 0, len(v.Stages))
	for _, s := range v.Stages {
		stages = append(stages, string(s))
	}
	return map[string]interface{}{
		"secret_id":      Str(v.SecretId),
		"version_number": Int64OrNil(v.VersionNumber),
		"name":           Str(v.Name),
		"content_type":   string(v.ContentType),
		"stages":         stages,
		"time_created":   FormatTime(v.TimeCreated),
	}
}

func SummariseSecretVersionSummary(v *ovault.SecretVersionSummary) map[string]interface{} {
	stages := make([]string, 0, len(v.Stages))
	for _, s := range v.Stages {
		stages = append(stages, string(s))
	}
	return map[string]interface{}{
		"secret_id":      Str(v.SecretId),
		"version_number": Int64OrNil(v.VersionNumber),
		"name":           Str(v.Name),
		"content_type":   string(v.ContentType),
		"stages":         stages,
		"time_created":   FormatTime(v.TimeCreated),
	}
}

func SummariseSecretBundleVersion(v *secrets.SecretBundleVersionSummary) map[string]interface{} {
	stages := make([]string, 0, len(v.Stages))
	for _, s := range v.Stages {
		stages = append(stages, string(s))
	}
	return map[string]interface{}{
		"secret_id":      Str(v.SecretId),
		"version_number": Int64OrNil(v.VersionNumber),
		"version_name":   Str(v.VersionName),
		"stages":         stages,
		"time_created":   FormatTime(v.TimeCreated),
	}
}

func SummariseSecret(s *ovault.Secret) map[string]interface{} {
	return map[string]interface{}{
		"id":                     Str(s.Id),
		"secret_name":            Str(s.SecretName),
		"description":            Str(s.Description),
		"compartment_id":         Str(s.CompartmentId),
		"vault_id":               Str(s.VaultId),
		"key_id":                 Str(s.KeyId),
		"current_version_number": Int64OrNil(s.CurrentVersionNumber),
		"lifecycle_state":        string(s.LifecycleState),
		"time_created":           FormatTime(s.TimeCreated),
	}
}

func SummariseSecretSummary(s *ovault.SecretSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              Str(s.Id),
		"secret_name":     Str(s.SecretName),
		"description":     Str(s.Description),
		"compartment_id":  Str(s.CompartmentId),
		"vault_id":        Str(s.VaultId),
		"key_id":          Str(s.KeyId),
		"lifecycle_state": string(s.LifecycleState),
		"time_created":    FormatTime(s.TimeCreated),
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
