// Package compute holds what every Azure Virtual Machines action shares: the
// service-principal credential and the Azure Resource Manager (ARM) client
// factory. Like sql/common.go and aws/common.go it has no Execute function, so
// the manifest generator skips it — but its category.go still supplies the
// "Virtual Machines" sub-category metadata.
//
// Every action needs an ARM client built from the operator's service-principal
// credentials, scoped to one subscription. GetAuth centralises that so the
// ~dozen VM/NSG/disk/snapshot actions don't each re-implement it. Note the
// manifest generator only resolves INLINE Inputs literals, so the credential +
// resource input *declarations* (azure_tenant_id, subscription_id, ...) must
// still be copy-pasted into each action's Inputs — only the Execute-side logic
// is shared.
//
// Auth is deliberately azidentity's, not hand-rolled: the storage/cosmos nodes
// learned the hard way that hand-rolling the Entra exchange is where a real
// MITM hole lived (see azure/common.go). azidentity.NewClientSecretCredential
// owns its own verifying pipeline; it is an azcore.TokenCredential the ARM SDK
// clients consume directly.
package compute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"

	core "flomation.app/automate/executor"
)

// Standard input names shared by every Azure Compute action.
const (
	InputTenantID       = "azure_tenant_id"
	InputClientID       = "azure_client_id"
	InputClientSecret   = "azure_client_secret"
	InputSubscriptionID = "subscription_id"
	InputResourceGroup  = "resource_group"
	InputLocation       = "location"
)

// Auth carries the service-principal credentials plus the subscription and
// resource-group scope every ARM call needs.
type Auth struct {
	TenantID       string
	ClientID       string
	ClientSecret   string
	SubscriptionID string
	ResourceGroup  string // required by most actions; a few list across the whole subscription
	Location       string // only the create actions need it
	cred           azcore.TokenCredential
}

// GetAuth reads the standard credential + scope input block and builds the
// service-principal credential. ResourceGroup and Location are read but not
// required here — actions that need them call RequiredString so the error names
// the field the operator left blank.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	a := Auth{
		TenantID:       strings.TrimSpace(OptionalString(InputTenantID, inputs)),
		ClientID:       strings.TrimSpace(OptionalString(InputClientID, inputs)),
		ClientSecret:   OptionalString(InputClientSecret, inputs),
		SubscriptionID: strings.TrimSpace(OptionalString(InputSubscriptionID, inputs)),
		ResourceGroup:  strings.TrimSpace(OptionalString(InputResourceGroup, inputs)),
		Location:       strings.TrimSpace(OptionalString(InputLocation, inputs)),
	}
	if a.TenantID == "" {
		return Auth{}, fmt.Errorf("tenant ID is required")
	}
	if a.ClientID == "" {
		return Auth{}, fmt.Errorf("client ID is required")
	}
	if a.ClientSecret == "" {
		return Auth{}, fmt.Errorf("client secret is required")
	}
	if a.SubscriptionID == "" {
		return Auth{}, fmt.Errorf("subscription ID is required")
	}
	// azidentity validates the tenant too, but its message is about its own API;
	// this one names the field the operator filled in.
	if strings.ContainsAny(a.TenantID, "/\\?#@ ") {
		return Auth{}, fmt.Errorf("tenant ID %q contains invalid characters", a.TenantID)
	}
	cred, err := azidentity.NewClientSecretCredential(a.TenantID, a.ClientID, a.ClientSecret, nil)
	if err != nil {
		return Auth{}, fmt.Errorf("failed to build Azure credential: %s", a.redact(err.Error()))
	}
	a.cred = cred
	return a, nil
}

// Credential exposes the underlying azcore.TokenCredential for the rare action
// that needs a client type without a factory method below.
func (a Auth) Credential() azcore.TokenCredential { return a.cred }

// ---------------------------------------------------------------------------
// ARM client factory — one authenticated client per resource type
// ---------------------------------------------------------------------------

func (a Auth) VMClient() (*armcompute.VirtualMachinesClient, error) {
	return armcompute.NewVirtualMachinesClient(a.SubscriptionID, a.cred, nil)
}

func (a Auth) NSGClient() (*armnetwork.SecurityGroupsClient, error) {
	return armnetwork.NewSecurityGroupsClient(a.SubscriptionID, a.cred, nil)
}

func (a Auth) SecurityRulesClient() (*armnetwork.SecurityRulesClient, error) {
	return armnetwork.NewSecurityRulesClient(a.SubscriptionID, a.cred, nil)
}

func (a Auth) DisksClient() (*armcompute.DisksClient, error) {
	return armcompute.NewDisksClient(a.SubscriptionID, a.cred, nil)
}

func (a Auth) SnapshotsClient() (*armcompute.SnapshotsClient, error) {
	return armcompute.NewSnapshotsClient(a.SubscriptionID, a.cred, nil)
}

func (a Auth) SSHKeysClient() (*armcompute.SSHPublicKeysClient, error) {
	return armcompute.NewSSHPublicKeysClient(a.SubscriptionID, a.cred, nil)
}

func (a Auth) ImagesClient() (*armcompute.ImagesClient, error) {
	return armcompute.NewImagesClient(a.SubscriptionID, a.cred, nil)
}

func (a Auth) TagsClient() (*armresources.TagsClient, error) {
	return armresources.NewTagsClient(a.SubscriptionID, a.cred, nil)
}

// ResourceGroupScope is the ARM resource-ID prefix for the auth's subscription
// + resource group — the parent every VM/NSG/disk lives under.
func (a Auth) ResourceGroupScope() string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", a.SubscriptionID, a.ResourceGroup)
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// OptionalString returns a string input's value, or "" when absent/unset.
func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return ""
	}
	if s := c.String(); s != nil {
		return *s
	}
	return ""
}

// RequiredString returns a trimmed input value, or an error naming the field.
func RequiredString(name string, inputs []*core.Connection) (string, error) {
	if v := strings.TrimSpace(OptionalString(name, inputs)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s is required", strings.ReplaceAll(name, "_", " "))
}

// RequiredResourceGroup is the common case: nearly every action is scoped to one
// resource group. Kept separate so the message reads "resource group is required".
func (a Auth) RequiredResourceGroup() (string, error) {
	if a.ResourceGroup == "" {
		return "", fmt.Errorf("resource group is required")
	}
	return a.ResourceGroup, nil
}

// InputStrings splits a comma-separated input into a trimmed, non-empty slice.
func InputStrings(name string, inputs []*core.Connection) []string {
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

// RequiredInt reads a whole-number input, erroring (naming the field) when it
// is blank or not an integer.
func RequiredInt(name string, inputs []*core.Connection) (int, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	label := strings.ReplaceAll(name, "_", " ")
	if raw == "" {
		return 0, fmt.Errorf("%s is required", label)
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a whole number", label)
	}
	return n, nil
}

// TagMap parses a JSON object input ({"env":"prod"}) into the *string map ARM
// wants. An empty/blank input yields (nil, nil); malformed JSON is an error.
func TagMap(name string, inputs []*core.Connection) (map[string]*string, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var flat map[string]string
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return nil, fmt.Errorf("tags must be a JSON object of string values, e.g. {\"env\":\"prod\"}: %s", err.Error())
	}
	out := make(map[string]*string, len(flat))
	for k, v := range flat {
		out[k] = strPtr(v)
	}
	return out, nil
}

func strPtr(s string) *string { return &s }

// Str safely dereferences an SDK *string field (ARM models are pointer-heavy),
// yielding "" for nil.
func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ---------------------------------------------------------------------------
// Result shaping & error classification
// ---------------------------------------------------------------------------

// ErrorResult is the soft-failure envelope: the flow continues, and the node
// reports success=false with a clean message on the error/tool_result outputs.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"success":     false,
		"error":       msg,
		"tool_result": msg,
	}
}

// AzureError turns an ARM SDK error into an operator-readable string. ARM
// surfaces failures as *azcore.ResponseError carrying the service error code
// (e.g. ResourceNotFound, AuthorizationFailed) and HTTP status; those name the
// real problem far better than the raw wire dump.
func (a Auth) AzureError(err error) string {
	if err == nil {
		return ""
	}
	var re *azcore.ResponseError
	if errors.As(err, &re) {
		code := re.ErrorCode
		if code == "" {
			code = fmt.Sprintf("HTTP %d", re.StatusCode)
		}
		return a.redact(fmt.Sprintf("Azure rejected the request (%s): %s", code, firstLine(re.Error())))
	}
	return a.redact(err.Error())
}

// AzureError without an Auth (credential-build failures happen before one
// exists). Redacts nothing since no secret is in scope yet.
func AzureError(err error) string {
	if err == nil {
		return ""
	}
	var re *azcore.ResponseError
	if errors.As(err, &re) {
		code := re.ErrorCode
		if code == "" {
			code = fmt.Sprintf("HTTP %d", re.StatusCode)
		}
		return fmt.Sprintf("Azure rejected the request (%s): %s", code, firstLine(re.Error()))
	}
	return err.Error()
}

// redact strips the client secret from any string bound for an output/log.
func (a Auth) redact(s string) string {
	if a.ClientSecret != "" {
		s = strings.ReplaceAll(s, a.ClientSecret, "REDACTED")
	}
	return s
}

// firstLine keeps error strings to their first meaningful line — azcore's
// ResponseError.Error() prints the whole request/response dump otherwise.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// context.Background is what every action passes; a tiny alias keeps the call
// sites uniform and makes the intent (no request deadline from the executor)
// explicit at each Execute.
func Context() context.Context { return context.Background() }
