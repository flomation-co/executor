package azure_storage_container_lease

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"
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
	call, err := storage.BuildLeaseCall(inputs)
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}

	resp, err := storage.Do(flow, auth, storage.Request{
		Method: http.MethodPut,
		Path:   storage.ContainerPath(container),
		// restype=container is what separates this from Lease Blob: the same
		// comp=lease against the same path leases the container only when the
		// resource type says so.
		Query:   url.Values{"restype": []string{"container"}, "comp": []string{"lease"}},
		Headers: call.Headers,
	})
	if err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	if err := storage.CheckResponse(resp); err != nil {
		return storage.ErrorResult(err.Error()), nil
	}
	return storage.LeaseResult(call, container, fmt.Sprintf("container %s", container), resp), nil
}
