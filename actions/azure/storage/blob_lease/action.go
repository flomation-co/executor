package azure_storage_blob_lease

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/lease"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure Storage: Lease Blob"
	Description  = "Take, extend, or release a write lock on a blob. Acquire returns a Lease ID — pass it to the Lease ID field of any Upload/Delete/Set step to write to the locked blob; every write WITHOUT it is refused while the lease is held. Break ends a lease you do not hold"
	Website      = "https://www.flomation.co"
	Icon         = "azure+lock"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account_name", Type: core.ConnectionTypeString, Label: "Storage Account", Placeholder: "mystorageaccount", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Shared Key", Value: "shared_key"}, {Name: "Microsoft Entra (service principal)", Value: "entra"}}},
	{Name: "account_key", Type: core.ConnectionTypeSecret, Label: "Account Key", Placeholder: "Base64 account key — Storage Account ▸ Access keys", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "shared_key"}}},
	{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Label: "Tenant ID", Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Application (client) ID of the service principal", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "The app needs a Storage Blob Data role on the account (RBAC)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}}},
	{Name: "endpoint", Type: core.ConnectionTypeString, Label: "Custom Endpoint", Placeholder: "https://myaccount.blob.core.windows.net — leave blank to derive; Azurite: http://host:10000/devstoreaccount1"},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate"},
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container", Placeholder: "my-container", Required: true},
	{Name: "blob_name", Type: core.ConnectionTypeString, Label: "Blob Name", Placeholder: "reports/2026/summary.pdf", Required: true},
	{
		Name:     "lease_action",
		Type:     core.ConnectionTypeString,
		Label:    "Lease Action",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Acquire — take the lock", Value: "acquire"},
			{Name: "Renew — extend the lock you hold", Value: "renew"},
			{Name: "Change — swap the lock's ID", Value: "change"},
			{Name: "Release — hand the lock back", Value: "release"},
			{Name: "Break — end someone else's lock", Value: "break"},
		},
	},
	{
		Name:        "lease_id",
		Type:        core.ConnectionTypeString,
		Label:       "Lease ID",
		Placeholder: "The Lease ID output of the Acquire step — optional on Break, which does not need it",
		Visible:     &core.VisibleWhen{Field: "lease_action", Values: []string{"renew", "change", "release", "break"}},
	},
	{
		Name:        "proposed_lease_id",
		Type:        core.ConnectionTypeString,
		Label:       "Proposed Lease ID",
		Placeholder: "A GUID to use as the lease's ID — leave blank on Acquire to let Azure choose one",
		Visible:     &core.VisibleWhen{Field: "lease_action", Values: []string{"acquire", "change"}},
	},
	{
		Name:        "duration",
		Type:        core.ConnectionTypeInteger,
		Label:       "Duration (seconds)",
		Placeholder: "15-60 seconds, or -1 to hold the lease until it is released",
		Value:       60,
		Visible:     &core.VisibleWhen{Field: "lease_action", Values: []string{"acquire"}},
	},
	{
		Name:        "break_period",
		Type:        core.ConnectionTypeInteger,
		Label:       "Break Period (seconds)",
		Placeholder: "0-60 — how long the lease may still run. 0 ends it immediately. Blank lets it run out its remaining time",
		Visible:     &core.VisibleWhen{Field: "lease_action", Values: []string{"break"}},
	},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Blob Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "lease_id", Type: core.ConnectionTypeString, Label: "Lease ID"},
	{Name: "lease_time", Type: core.ConnectionTypeInteger, Label: "Lease Time Remaining (seconds)"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := storage.GetAuth(inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	container, err := storage.RequiredString("container", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	blobName, err := storage.RequiredString("blob_name", inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	// BuildLeaseCall still validates the action, the duration bounds and the
	// proposed-lease-id GUID form — only the transport moves to the SDK. Its
	// x-ms-* Headers are unused here; the lease subpackage sets them itself.
	call, err := storage.BuildLeaseCall(inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	bc, err := auth.BlobClient(container, blobName)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	ctx := flow.GoContext()

	// fail redacts and shapes any SDK/client error the same way across branches.
	fail := func(err error) (map[string]interface{}, error) {
		_, msg := auth.SDKError(err)
		return storage.ErrorResult(msg), nil
	}

	// The lease subpackage carries the lease ID on the CLIENT (as proposed-id on
	// acquire, current-id on renew/change/release), so each action builds its
	// lease client from the right ID. leaseID/leaseTime mirror the two headers
	// LeaseResult reads: x-ms-lease-id and x-ms-lease-time.
	var (
		leaseID   string
		leaseTime int
		etag      *azcore.ETag
		lm        *time.Time
	)

	switch call.Action {
	case storage.LeaseAcquire:
		// Blank proposed ⇒ nil options ⇒ the SDK mints a client-side GUID and
		// sends it as x-ms-proposed-lease-id; the response echoes it back as the
		// lease ID, so lease_id is populated exactly as the server-mint path was.
		var opts *lease.BlobClientOptions
		if proposed := storage.OptionalString("proposed_lease_id", inputs); proposed != "" {
			opts = &lease.BlobClientOptions{LeaseID: &proposed}
		}
		lc, err := lease.NewBlobClient(bc, opts)
		if err != nil {
			return fail(err)
		}
		resp, err := lc.AcquireLease(ctx, int32(call.Duration), nil)
		if err != nil {
			return fail(err)
		}
		if resp.LeaseID != nil {
			leaseID = *resp.LeaseID
		}
		etag, lm = resp.ETag, resp.LastModified

	case storage.LeaseRenew:
		currentID := storage.OptionalString("lease_id", inputs)
		lc, err := lease.NewBlobClient(bc, &lease.BlobClientOptions{LeaseID: &currentID})
		if err != nil {
			return fail(err)
		}
		resp, err := lc.RenewLease(ctx, nil)
		if err != nil {
			return fail(err)
		}
		if resp.LeaseID != nil {
			leaseID = *resp.LeaseID
		}
		etag, lm = resp.ETag, resp.LastModified

	case storage.LeaseChange:
		currentID := storage.OptionalString("lease_id", inputs)
		proposed := storage.OptionalString("proposed_lease_id", inputs)
		lc, err := lease.NewBlobClient(bc, &lease.BlobClientOptions{LeaseID: &currentID})
		if err != nil {
			return fail(err)
		}
		resp, err := lc.ChangeLease(ctx, proposed, nil)
		if err != nil {
			return fail(err)
		}
		if resp.LeaseID != nil {
			leaseID = *resp.LeaseID
		}
		etag, lm = resp.ETag, resp.LastModified

	case storage.LeaseRelease:
		currentID := storage.OptionalString("lease_id", inputs)
		lc, err := lease.NewBlobClient(bc, &lease.BlobClientOptions{LeaseID: &currentID})
		if err != nil {
			return fail(err)
		}
		resp, err := lc.ReleaseLease(ctx, nil)
		if err != nil {
			return fail(err)
		}
		// Release returns no x-ms-lease-id — the lease is gone; leaseID stays "".
		etag, lm = resp.ETag, resp.LastModified

	case storage.LeaseBreak:
		// Break needs no lease ID; only the optional break period. -1 (unset)
		// means the service uses the remaining lease time.
		var opts *lease.BlobBreakOptions
		if call.BreakPeriod >= 0 {
			period := int32(call.BreakPeriod)
			opts = &lease.BlobBreakOptions{BreakPeriod: &period}
		}
		lc, err := lease.NewBlobClient(bc, nil)
		if err != nil {
			return fail(err)
		}
		resp, err := lc.BreakLease(ctx, opts)
		if err != nil {
			return fail(err)
		}
		// x-ms-lease-time: seconds a broken lease still has to run (0 ⇒ gone).
		if resp.LeaseTime != nil {
			leaseTime = int(*resp.LeaseTime)
		}
		etag, lm = resp.ETag, resp.LastModified
	}

	props := map[string]interface{}{}
	if etag != nil {
		props["etag"] = string(*etag)
	}
	if lm != nil {
		props["lastModified"] = lm.UTC().Format(time.RFC1123)
	}

	// Summary strings reproduced verbatim from storage.LeaseResult.
	target := blobName
	var summary string
	switch call.Action {
	case storage.LeaseAcquire:
		if call.Duration == storage.LeaseInfiniteDuration {
			summary = fmt.Sprintf("Acquired an infinite lease on %s", target)
		} else {
			summary = fmt.Sprintf("Acquired a %ds lease on %s", call.Duration, target)
		}
	case storage.LeaseRenew:
		summary = fmt.Sprintf("Renewed the lease on %s", target)
	case storage.LeaseChange:
		summary = fmt.Sprintf("Changed the lease ID on %s", target)
	case storage.LeaseRelease:
		summary = fmt.Sprintf("Released the lease on %s", target)
	case storage.LeaseBreak:
		if leaseTime > 0 {
			summary = fmt.Sprintf("Broke the lease on %s — it ends in %ds", target, leaseTime)
		} else {
			summary = fmt.Sprintf("Broke the lease on %s", target)
		}
	}

	result := map[string]interface{}{"name": blobName, "properties": props, "leaseAction": call.Action}
	out := storage.ResourceResult(blobName, result, summary)
	out["lease_id"] = leaseID
	out["lease_time"] = leaseTime
	return out, nil
}
