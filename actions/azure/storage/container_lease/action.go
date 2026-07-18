package azure_storage_container_lease

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
	Name         = "Azure Storage: Lease Container"
	Description  = "Take, extend, or release a lock on a container. A container lease guards the container itself — deleting it, or changing its metadata — not the blobs inside it. Acquire returns a Lease ID to pass to the Lease ID field of a Delete Container or Set Container Metadata step"
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
	{Name: "container", Type: core.ConnectionTypeString, Label: "Container Name", Placeholder: "my-container", Required: true},
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
	{Name: "id", Type: core.ConnectionTypeString, Label: "Container Name"},
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
	// BuildLeaseCall still validates every combination (durations, GUIDs, which
	// action needs which id) with the same messages; we read the action and the
	// two numbers it resolved, and take the raw ids for the lease client.
	call, err := storage.BuildLeaseCall(inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	leaseID := storage.OptionalString("lease_id", inputs)
	proposed := storage.OptionalString("proposed_lease_id", inputs)

	cc, err := auth.ContainerClient(container)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	// The lease client is seeded with the id the operation acts on: the proposed
	// id on Acquire (blank ⇒ the SDK mints one, as the service used to), the held
	// id on Renew/Change/Release. Break needs none.
	var clientLeaseID *string
	switch call.Action {
	case storage.LeaseAcquire:
		if proposed != "" {
			clientLeaseID = &proposed
		}
	case storage.LeaseRenew, storage.LeaseRelease, storage.LeaseChange:
		clientLeaseID = &leaseID
	}
	lc, err := lease.NewContainerClient(cc, &lease.ContainerClientOptions{LeaseID: clientLeaseID})
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	ctx := flow.GoContext()
	var (
		outETag    *azcore.ETag
		outLastMod *time.Time
		outLeaseID *string
		leaseTime  int
		callErr    error
	)
	switch call.Action {
	case storage.LeaseAcquire:
		resp, err := lc.AcquireLease(ctx, int32(call.Duration), nil)
		if callErr = err; err == nil {
			outETag, outLastMod, outLeaseID = resp.ETag, resp.LastModified, resp.LeaseID
		}
	case storage.LeaseRenew:
		resp, err := lc.RenewLease(ctx, nil)
		if callErr = err; err == nil {
			outETag, outLastMod, outLeaseID = resp.ETag, resp.LastModified, resp.LeaseID
		}
	case storage.LeaseChange:
		resp, err := lc.ChangeLease(ctx, proposed, nil)
		if callErr = err; err == nil {
			outETag, outLastMod, outLeaseID = resp.ETag, resp.LastModified, resp.LeaseID
		}
	case storage.LeaseRelease:
		resp, err := lc.ReleaseLease(ctx, nil)
		if callErr = err; err == nil {
			outETag, outLastMod = resp.ETag, resp.LastModified
		}
	case storage.LeaseBreak:
		var breakOpts *lease.ContainerBreakOptions
		// A specified break_period (0-60) travels; blank lets the lease run out
		// its remaining time. call.BreakPeriod is -1 when unset.
		if call.BreakPeriod >= 0 {
			bp := int32(call.BreakPeriod)
			breakOpts = &lease.ContainerBreakOptions{BreakPeriod: &bp}
		}
		resp, err := lc.BreakLease(ctx, breakOpts)
		if callErr = err; err == nil {
			outETag, outLastMod = resp.ETag, resp.LastModified
			if resp.LeaseTime != nil {
				leaseTime = int(*resp.LeaseTime)
			}
		}
	}
	if callErr != nil {
		_, msg := auth.SDKError(callErr)
		return storage.ErrorResult(msg), nil
	}

	// Reproduce LeaseResult's output: the standard resource envelope plus the two
	// things a downstream node needs — lease_id (the whole point) and lease_time
	// (the seconds a break still has to run).
	target := fmt.Sprintf("container %s", container)
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

	props := map[string]interface{}{}
	if outETag != nil {
		props["etag"] = string(*outETag)
	}
	if outLastMod != nil {
		props["lastModified"] = outLastMod.UTC().Format(time.RFC1123)
	}
	result := map[string]interface{}{"name": container, "properties": props, "leaseAction": call.Action}

	out := storage.ResourceResult(container, result, summary)
	leaseIDStr := ""
	if outLeaseID != nil {
		leaseIDStr = *outLeaseID
	}
	out["lease_id"] = leaseIDStr
	out["lease_time"] = leaseTime
	return out, nil
}
