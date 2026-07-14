// Package infrastructure_awx_project_create adds a source-control project to AWX.
//
// Two things are enforced client-side, because AWX's own failures for them are
// baffling:
//
//   - ORGANIZATION IS REQUIRED. DRF says it is optional, but
//     ProjectAccess.can_add uses check_related(mandatory=True) — so for a
//     NON-SUPERUSER, omitting it is a 403 PermissionDenied ("you may not create
//     projects"), not a 400 field error. Requiring it up front turns an
//     unexplainable permissions error into an obvious empty field.
//   - SOURCE CONTROL URL IS REQUIRED once a source-control type is chosen.
package infrastructure_awx_project_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Create Project"
	Description  = "Add a source-control project to AWX so its playbooks can be used by job templates."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+plus"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// --- AWX credential block (identical in all 59 AWX actions) ---
	{Name: "awx_url", Type: core.ConnectionTypeString, Label: "AWX / AAP URL", Placeholder: "https://awx.example.com — your AWX or Ansible Automation Platform address", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{
		{Name: "API Token (recommended)", Value: "token"},
		{Name: "Username & Password", Value: "basic"},
	}},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "AWX ▸ your user ▸ Tokens ▸ Add, Application blank, Scope = Write. Shown once — copy it then.", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token"}}},
	{Name: "awx_username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your AWX username", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"basic"}}},
	{Name: "awx_password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "your AWX password — note some AWX installs disable password authentication", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"basic"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip certificate verification — only for a self-hosted AWX with a self-signed certificate"},
	{Name: "api_prefix", Type: core.ConnectionTypeString, Label: "API Path Prefix (advanced)", Placeholder: "Leave blank — detected automatically. Only set this if support asks (e.g. /api/controller/v2/)."},

	// --- the project ---
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Acme Playbooks", Required: true},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization", Placeholder: "Choose the organization to own this project, or enter its AWX ID", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this project is for"},
	{Name: "scm_type", Type: core.ConnectionTypeString, Label: "Source Control Type", Options: []core.ConnectionOption{
		{Name: "Git", Value: "git"},
		{Name: "Subversion", Value: "svn"},
		{Name: "Remote Archive", Value: "archive"},
		{Name: "Insights", Value: "insights"},
		{Name: "Manual (no source control)", Value: ""},
	}},
	{Name: "scm_url", Type: core.ConnectionTypeString, Label: "Source Control URL", Placeholder: "https://github.com/acme/playbooks.git — required unless the type is Manual"},
	{Name: "scm_branch", Type: core.ConnectionTypeString, Label: "Branch / Tag / Commit", Placeholder: "main — leave blank for the repository's default branch"},
	{Name: "scm_refspec", Type: core.ConnectionTypeString, Label: "Source Control Refspec", Placeholder: "refs/pull/*:refs/remotes/origin/pull/* — advanced, Git only", Visible: &core.VisibleWhen{Field: "scm_type", Values: []string{"git"}}},
	{Name: "credential_id", Type: core.ConnectionTypeString, Label: "Source Control Credential", Placeholder: "A Source Control credential — needed for private repositories"},
	{Name: "scm_clean", Type: core.ConnectionTypeBoolean, Label: "Clean", Placeholder: "Discard any local changes before each sync"},
	{Name: "scm_delete_on_update", Type: core.ConnectionTypeBoolean, Label: "Delete", Placeholder: "Delete the local copy before each sync and clone it again"},
	{Name: "scm_track_submodules", Type: core.ConnectionTypeBoolean, Label: "Track Submodules", Placeholder: "Follow each submodule's own branch rather than the pinned commit — Git only", Visible: &core.VisibleWhen{Field: "scm_type", Values: []string{"git"}}},
	{Name: "scm_update_on_launch", Type: core.ConnectionTypeBoolean, Label: "Update On Launch", Placeholder: "Sync from source control every time a job template using this project is launched"},
	{Name: "scm_update_cache_timeout", Type: core.ConnectionTypeInteger, Label: "Cache Timeout (seconds)", Placeholder: "With Update On Launch, skip the sync if one ran within this many seconds — default 0"},
	{Name: "allow_override", Type: core.ConnectionTypeBoolean, Label: "Allow job templates to override the branch"},
	{Name: "sync_timeout", Type: core.ConnectionTypeInteger, Label: "Sync Timeout (seconds)", Placeholder: "Cancel a sync that runs longer than this — 0 means no limit"},
	{Name: "default_environment_id", Type: core.ConnectionTypeString, Label: "Default Execution Environment", Placeholder: "The execution environment jobs using this project run in — leave blank for the organization's default"},
	{Name: "local_path", Type: core.ConnectionTypeString, Label: "Playbook Directory", Placeholder: "The folder under /var/lib/awx/projects on the AWX server holding the playbooks", Visible: &core.VisibleWhen{Field: "scm_type", Values: []string{""}}},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"signature_validation_credential": 3} — any other AWX project field; these override the fields above`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Project ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Project"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err // the ONE hard failure: the node is mis-configured
	}

	ctx, cancel := awx.Context()
	defer cancel()

	name, err := awx.RequiredString("name", "Name", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	orgID, err := awx.RequiredInt("organization_id", "Organization", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	scmType := awx.OptionalString("scm_type", inputs)
	scmURL := awx.OptionalString("scm_url", inputs)
	if scmType != "" && scmURL == "" {
		return awx.ErrorResult(fmt.Sprintf(
			"Source Control URL is required for a %s project — e.g. https://github.com/acme/playbooks.git. (Choose Manual as the Source Control Type if the playbooks already live on the AWX server.)", scmType)), nil
	}

	body := map[string]interface{}{
		"name":         name,
		"organization": orgID,
		// Always sent, empty string included: "" IS AWX's value for a manual
		// project, and it is also AWX's default, so an untouched dropdown means
		// the same thing either way.
		"scm_type": scmType,
	}

	awx.SetIfPresent(body, inputs, "description", "description")
	awx.SetIfPresent(body, inputs, "scm_url", "scm_url")
	awx.SetIfPresent(body, inputs, "scm_branch", "scm_branch")
	awx.SetIfPresent(body, inputs, "scm_refspec", "scm_refspec")
	awx.SetIfPresent(body, inputs, "local_path", "local_path")

	// Tri-state: an untouched checkbox is omitted rather than sent as false. It
	// matters here — with scm_type="" AWX REFUSES any of these set to true, so an
	// omitted key is the one that cannot cause a spurious 400.
	awx.SetBoolIfSet(body, inputs, "scm_clean", "scm_clean")
	awx.SetBoolIfSet(body, inputs, "scm_delete_on_update", "scm_delete_on_update")
	awx.SetBoolIfSet(body, inputs, "scm_track_submodules", "scm_track_submodules")
	awx.SetBoolIfSet(body, inputs, "scm_update_on_launch", "scm_update_on_launch")
	awx.SetBoolIfSet(body, inputs, "allow_override", "allow_override")

	if err := awx.SetIntIfPresent(body, inputs, "credential", "credential_id"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.SetIntIfPresent(body, inputs, "default_environment", "default_environment_id"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.SetIntIfPresent(body, inputs, "scm_update_cache_timeout", "scm_update_cache_timeout"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	// AWX names the project's sync timeout plainly "timeout"; the input is
	// sync_timeout so it cannot be mistaken for a job template's timeout.
	if err := awx.SetIntIfPresent(body, inputs, "timeout", "sync_timeout"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// LAST, so a power user's raw field wins over the first-class inputs above.
	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	project, err := awx.CreateResource(ctx, auth, "projects/", body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.ObjectResult(project, fmt.Sprintf("Created project %q (%s)",
		awx.StringField(project, "name"), awx.IDString(project["id"]))), nil
}
