package storage

// sdk.go holds the azblob-SDK client factory and the SDK-native helpers the
// actions call. It replaces the hand-rolled SharedKey signing that used to live
// in common.go (Do/Request/CheckResponse/sharedKeyStringToSign): the SDK owns
// the string-to-sign now, which is the whole point of the migration — the 13-slot
// Blob signature is exactly the kind of thing that flat-403'd against Azurite in
// wave 1 and is better left to code Microsoft maintains and tests.
//
// The client/transport/Entra pattern mirrors the Tables node (actions/azure/
// tables/common.go), which adopted aztables first: one authenticated
// service.Client, everything (containers, blobs, block blobs, SAS) derived from
// it, a custom transport so the opt-in allow_insecure path and Azurite's
// path-style endpoint both work, and the azidentity-backed token mint adapted
// to azcore.TokenCredential.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"

	azure "flomation.app/automate/executor/actions/azure"
)

// ---------------------------------------------------------------------------
// Transport + client options
// ---------------------------------------------------------------------------

// transportFor returns the shared secure client, or the opt-in insecure one
// (InsecureSkipVerify) when the node's allow_insecure box is ticked — used only
// for a self-signed custom endpoint. Kept as two distinct clients so the secure
// default can never be weakened by a config flag.
func transportFor(a Auth) policy.Transporter {
	if a.Insecure {
		return insecureHTTPClient
	}
	return httpClient
}

func serviceClientOptions(a Auth) *service.ClientOptions {
	return &service.ClientOptions{ClientOptions: azcore.ClientOptions{Transport: transportFor(a)}}
}

// ---------------------------------------------------------------------------
// Client factory — one authenticated service.Client, everything derives from it
// ---------------------------------------------------------------------------

// ServiceClient builds the account-level client. a.BaseURL is already
// normalised by GetAuth (scheme+host, plus the account path for Azurite-style
// endpoints), so the same call works against real Azure and the emulator.
func (a Auth) ServiceClient() (*service.Client, error) {
	switch a.Method {
	case AuthEntra:
		cred := entraCredential{tenantID: a.TenantID, clientID: a.ClientID, clientSecret: a.ClientSecret}
		c, err := service.NewClient(a.BaseURL, cred, serviceClientOptions(a))
		if err != nil {
			return nil, fmt.Errorf("failed to build the Blob service client: %s", a.redact(err.Error()))
		}
		return c, nil
	default:
		cred, err := a.SharedKeyCredential()
		if err != nil {
			return nil, err
		}
		c, err := service.NewClientWithSharedKeyCredential(a.BaseURL, cred, serviceClientOptions(a))
		if err != nil {
			return nil, fmt.Errorf("failed to build the Blob service client: %s", a.redact(err.Error()))
		}
		return c, nil
	}
}

// SharedKeyCredential builds the SDK credential from the pasted key. It is
// exported because SAS generation needs it directly (a service SAS is signed
// with the account key; an Entra principal has no key to sign with).
func (a Auth) SharedKeyCredential() (*service.SharedKeyCredential, error) {
	cred, err := service.NewSharedKeyCredential(a.AccountName, a.rawKey)
	if err != nil {
		return nil, fmt.Errorf("account_key is not valid: %s", a.redact(err.Error()))
	}
	return cred, nil
}

// ContainerClient / BlobClient / BlockBlobClient derive a scoped client from the
// service client. Names are validated by the caller (ValidateContainerName /
// BlobPath) before they reach here, so a bad name fails by rule rather than as
// an opaque 400.
func (a Auth) ContainerClient(name string) (*container.Client, error) {
	svc, err := a.ServiceClient()
	if err != nil {
		return nil, err
	}
	return svc.NewContainerClient(name), nil
}

func (a Auth) BlobClient(containerName, blobName string) (*blob.Client, error) {
	cc, err := a.ContainerClient(containerName)
	if err != nil {
		return nil, err
	}
	return cc.NewBlobClient(blobName), nil
}

func (a Auth) BlockBlobClient(containerName, blobName string) (*blockblob.Client, error) {
	cc, err := a.ContainerClient(containerName)
	if err != nil {
		return nil, err
	}
	return cc.NewBlockBlobClient(blobName), nil
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

// SDKError turns an azblob error into (code, redacted-message). The code is the
// service's own ErrorCode string (e.g. "BlobAlreadyExists", "ContainerNotFound")
// so actions can branch on it exactly as they did on ErrorCode() before; the
// message is scrubbed of any credential material before it can reach an output.
func (a Auth) SDKError(err error) (code, message string) {
	if err == nil {
		return "", ""
	}
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.ErrorCode, a.redact(err.Error())
	}
	return "", a.redact(err.Error())
}

// HasCode reports whether err is the given azblob error code.
func HasCode(err error, code bloberror.Code) bool {
	return bloberror.HasCode(err, code)
}

// ---------------------------------------------------------------------------
// SDK-native output shaping
// ---------------------------------------------------------------------------
//
// The pre-SDK actions built their `result` object from raw response headers via
// HeadersResult, producing {name, properties:{etag,lastModified,...}, metadata}.
// The SDK exposes those as typed fields instead, so these helpers rebuild the
// same shape from typed responses. The REFERENCEABLE output surface — the
// top-level keys id/result/tool_result/success/error (+etag/url where an action
// adds them) — is unchanged; only the internals of the opaque `result` object
// move from header-case to SDK-native, which nothing in the editor references
// individually.

// etagString renders an *azcore.ETag as the quoted string the header form used.
func etagString(e *azcore.ETag) string {
	if e == nil {
		return ""
	}
	return string(*e)
}

// WriteResult shapes the response of a create/write/set operation — the common
// case, where the service returns an ETag and Last-Modified and nothing an
// operator references beyond them. It sets both the nested properties and the
// top-level `etag` output (several write actions surfaced `etag` at top level).
func WriteResult(id string, etag *azcore.ETag, lastModified *time.Time, summary string) map[string]interface{} {
	props := map[string]interface{}{}
	if etag != nil {
		props["etag"] = etagString(etag)
	}
	if lastModified != nil {
		props["lastModified"] = lastModified.UTC().Format(time.RFC1123)
	}
	out := ResourceResult(id, map[string]interface{}{"name": id, "properties": props}, summary)
	if etag != nil {
		out["etag"] = etagString(etag)
	}
	return out
}

// PropsResult shapes a read operation: the action assembles the properties and
// metadata maps from the SDK's typed response, and this wraps them in the same
// {name, properties, metadata} envelope HeadersResult produced.
func PropsResult(id, summary string, props, metadata map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{"name": id, "properties": props}
	if len(metadata) > 0 {
		result["metadata"] = metadata
	}
	return ResourceResult(id, result, summary)
}

// StrMeta flattens the SDK's map[string]*string metadata into the
// map[string]interface{} the output envelope carries, LOWERCASING every key.
//
// The lowercasing is deliberate fidelity, not a stylistic choice. The pre-SDK
// node returned metadata keys lowercased everywhere (its HeadersResult
// lowercased the whole x-ms-meta-<name> header before extracting the name), so
// an operator's flow that reads result.metadata.archived expects that key. The
// Go SDK is inconsistent on its own: GetProperties and Download preserve the
// case Azure stored, but List (NewListBlobsFlatPager) lowercases — a known
// Go-SDK quirk, since Azure itself preserves case on both paths (verified
// against the real service via the CLI). Normalising to lowercase here keeps
// all three actions agreeing AND matches what the shipped node always returned.
// Metadata keys are case-insensitive in Azure, so no information is lost.
func StrMeta(meta map[string]*string) map[string]interface{} {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(meta))
	for k, v := range meta {
		if v != nil {
			out[strings.ToLower(k)] = *v
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// SDK-native input adapters
// ---------------------------------------------------------------------------

// MetadataMap reads the metadata JSON/object input into the map[string]*string
// the SDK wants (nil-value pointers are never produced — every value is set).
// Returns nil when the input is absent, which the SDK treats as "no metadata".
func MetadataMap(inputs []*core.Connection, inputName string) (map[string]*string, error) {
	raw, err := StringMapInput(inputName, inputs)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]*string, len(raw))
	for k, v := range raw {
		if !metadataNameRe.MatchString(k) {
			return nil, fmt.Errorf("metadata key %q is not a valid C# identifier (letters, digits, underscore; not starting with a digit)", k)
		}
		val := v
		out[k] = &val
	}
	return out, nil
}

// LeaseIDPtr reads the optional lease_id input, returning nil when absent so an
// action can pass it straight into a *blob.LeaseAccessConditions. An operator
// only supplies it to act on a blob someone else has leased.
func LeaseIDPtr(inputs []*core.Connection) *string {
	id := OptionalString("lease_id", inputs)
	if id == "" {
		return nil
	}
	return &id
}

// entraTokenFloor — see entraCredential.
const entraTokenFloor = time.Minute

// entraCredential adapts the shared azidentity-backed token mint
// (actions/azure/common.go) to the azcore.TokenCredential interface the azblob
// clients want.
//
// This is duplicated from the Tables node deliberately: both need the identical
// adapter, but hoisting it into the shared azure package would pull the tables
// change into this MR and force a re-validation of the Tables Entra path that
// the torn-down Azure account can no longer provide. A follow-up can unify them
// once there is a real account to validate against.
//
// ExpiresOn is a one-minute floor rather than the real expiry, which the mint
// does not report: claiming a longer life could hand the SDK a token already
// dead, and re-asking costs nothing — ClientCredentialsToken lands in
// azidentity's own per-scope cache, so a re-ask is a map lookup.
type entraCredential struct {
	tenantID     string
	clientID     string
	clientSecret string
}

func (c entraCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	scope := EntraScope
	if len(opts.Scopes) > 0 && opts.Scopes[0] != "" {
		scope = opts.Scopes[0]
	}
	token, err := azure.ClientCredentialsToken(ctx, c.tenantID, c.clientID, c.clientSecret, scope)
	if err != nil {
		return azcore.AccessToken{}, err
	}
	return azcore.AccessToken{Token: token, ExpiresOn: nowFunc().Add(entraTokenFloor)}, nil
}
