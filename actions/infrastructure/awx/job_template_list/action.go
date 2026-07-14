// Package infrastructure_awx_job_template_list lists the job templates on an
// AWX / AAP controller.
//
// The one thing this action does beyond a plain list: it reports, per template,
// whether the credential in use may actually LAUNCH it. AWX carries that in
// summary_fields.user_capabilities.start, and a template a user can SEE is very
// often one they cannot RUN — a read-scoped token, or a user without the Execute
// role, lists every template happily and only fails at the launch. Surfacing it
// in tool_result is what stops an AI agent picking a template it will then be
// refused on.
package infrastructure_awx_job_template_list

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Job Templates"
	Description  = "List the job templates on your AWX / AAP controller, optionally filtered by name, project or inventory."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+list"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

// maxSummaryItems bounds how many templates the tool_result names one by one.
// A Return All walk can pull thousands; an AI agent's context cannot.
const maxSummaryItems = 25

var Inputs = [...]core.Connection{
	// ---- AUTH (the shared block — see awx.AuthInputs; re-declared verbatim
	// because the manifest generator AST-parses this literal and cannot see
	// through a package-level variable) ----
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

	// ---- Filters ----
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Free-text search across name and description"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Exact template name — use Search for a partial match"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "Only templates that run playbooks from this project"},
	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Only templates that target this inventory"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Options: []core.ConnectionOption{
		{Name: "Name (A-Z)", Value: "name"},
		{Name: "Name (Z-A)", Value: "-name"},
		{Name: "Newest first", Value: "-created"},
		{Name: "Oldest first", Value: "created"},
		{Name: "Most recently run", Value: "-last_job_run"},
	}, Placeholder: "Default: Name (A-Z)"},

	// ---- Paging ----
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Which page of results to fetch — default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "How many per page — default 50, maximum 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow every page until all matching templates are fetched"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Job Templates"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_count", Type: core.ConnectionTypeInteger, Label: "Total Matching"},
	{Name: "has_more", Type: core.ConnectionTypeBoolean, Label: "More Available"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		// The ONLY hard error in the action: the node is mis-configured, not the
		// request. Everything below is a soft failure.
		return nil, err
	}

	ctx, cancel := awx.Context()
	defer cancel()

	// AWX's own default ordering here is by id; a human wants it by name.
	q, returnAll := awx.ListParams(inputs, "name")
	awx.AddFilter(q, inputs, "search", "search")
	awx.AddFilter(q, inputs, "name", "name")
	awx.AddFilter(q, inputs, "project", "project_id")
	awx.AddFilter(q, inputs, "inventory", "inventory_id")

	items, total, hasMore, err := awx.List(ctx, auth, "job_templates/", q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.ListResult(items, total, hasMore, summarise(items, total, hasMore)), nil
}

// summarise names each template and — the point of this action — says whether
// this AWX user may actually launch it.
func summarise(items []interface{}, total int, hasMore bool) string {
	if len(items) == 0 {
		return "No job templates matched. Check the filters, or check this AWX user can see them at all — AWX hides templates you have no role on rather than refusing them."
	}

	named := make([]string, 0, maxSummaryItems)
	unlaunchable := 0
	for i, raw := range items {
		obj, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		launchable := canStart(obj)
		if !launchable {
			unlaunchable++
		}
		if i < maxSummaryItems {
			state := "launchable"
			if !launchable {
				state = "NOT launchable by this AWX user"
			}
			named = append(named, fmt.Sprintf("%s %q (%s)", awx.IDString(obj["id"]), awx.StringField(obj, "name"), state))
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d job template(s)", len(items))
	if total > len(items) {
		fmt.Fprintf(&b, " of %d matching", total)
	}
	fmt.Fprintf(&b, ": %s", strings.Join(named, "; "))
	if len(items) > len(named) {
		fmt.Fprintf(&b, "; … and %d more", len(items)-len(named))
	}
	b.WriteString(".")

	if unlaunchable > 0 {
		fmt.Fprintf(&b, " %d of them are NOT launchable with this credential — the token is read-scoped, or this user has no Execute role on the template — so do not choose one of those to launch; AWX will refuse it with a 403.", unlaunchable)
	}
	if hasMore {
		b.WriteString(" More results remain: raise Page Size, ask for the next Page, or tick Return All.")
	}
	return b.String()
}

// canStart reads summary_fields.user_capabilities.start — AWX's own answer to
// "may the caller run this?". Absent (an older AWX, or a serializer that omits
// summary_fields) reads as false, which is the safe direction: the action says
// "not launchable" and the operator finds out here rather than at the launch.
func canStart(obj map[string]interface{}) bool {
	summary, _ := obj["summary_fields"].(map[string]interface{})
	caps, _ := summary["user_capabilities"].(map[string]interface{})
	return awx.BoolField(caps, "start")
}
