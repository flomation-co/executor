// Package infrastructure_awx_adhoc_command_run runs a single Ansible module
// against the hosts of an AWX inventory — an ad-hoc command.
//
// ★ This is the most dangerous action in the node. An ad-hoc command is a shell
// on every host in an inventory (`shell: rm -rf /` is a legal module_args), so it
// carries the mandatory confirm_destructive guard, LAST and Required, exactly as
// job_cancel and the delete actions do.
//
// Four AWX behaviours the operator would otherwise hit as an opaque 400:
//
//   - The MODULE ALLOW-LIST. module_name must be in AWX's AD_HOC_COMMANDS setting
//     (19 modules by default). It is an ADMIN-EDITABLE RUNTIME SETTING, so the
//     node does not ship a hardcoded list to validate against — the editor's live
//     dropdown reads the instance's own list from {root}settings/jobs/. What IS
//     validated here is the mistake every Ansible user makes: a fully-qualified
//     collection name. AWX takes SHORT NAMES ONLY, so `ansible.builtin.shell` is
//     rejected however the admin has configured the allow-list.
//   - module_args is REQUIRED AND NON-EMPTY for the command and shell modules, and
//     meaningless for ping. Checked client-side so the message names the field.
//   - extra_vars containing ANY ansible_* variable is rejected OUTRIGHT by AWX.
//   - ★ STALE STATUS. The 201 is serialised BEFORE signal_start() is called, so
//     the status it reports is a stale "new" — the opposite of a job-template
//     launch, which reports "pending". The record is therefore always re-fetched
//     before it is emitted, so a flow never branches on a status that was already
//     wrong when it was written.
//
// Ad-hoc commands have no artifacts, so none is emitted.
package infrastructure_awx_adhoc_command_run

import (
	"fmt"
	"sort"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "AWX: Run Ad-Hoc Command"
	Description  = "Run a single Ansible module against the hosts in an inventory — ping them, restart a service, or run a shell command. This executes immediately on the target hosts."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+terminal"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

// defaultModule is the module AWX's own ad-hoc UI defaults to. It lives here, not
// in the Inputs literal, because the manifest does not harvest Value — a default
// in the literal would be invisible to the editor.
const defaultModule = "command"

// argsRequiredModules are the modules whose whole payload IS module_args: AWX
// rejects an empty one with 400 {"module_args":["No argument passed to command
// module."]}.
var argsRequiredModules = map[string]bool{"command": true, "shell": true}

var Inputs = [...]core.Connection{
	// --- AUTH BLOCK (verbatim from awx.AuthInputs; the manifest AST-parser cannot
	// see through a package-level var, so all 59 actions re-declare it) ---
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

	// --- What to run, and where ---
	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "The inventory whose hosts the module runs against", Required: true},
	{Name: "credential_id", Type: core.ConnectionTypeString, Label: "Machine Credential", Placeholder: "The SSH / machine credential AWX logs into the hosts with — an ad-hoc command cannot run without one", Required: true},
	{Name: "module_name", Type: core.ConnectionTypeString, Label: "Module", Placeholder: "command (the default) · shell · ping · service · yum · apt · setup · win_ping … — short names only, never ansible.builtin.shell", Required: true},
	{Name: "module_args", Type: core.ConnectionTypeString, Label: "Module Arguments", Placeholder: "systemctl restart nginx — required for the command and shell modules"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "web*:&prod — blank targets EVERY host in the inventory"},

	// --- How to run it ---
	{Name: "job_type", Type: core.ConnectionTypeString, Label: "Job Type", Options: []core.ConnectionOption{
		{Name: "Run", Value: "run"},
		{Name: "Check (dry run)", Value: "check"},
	}},
	{Name: "verbosity", Type: core.ConnectionTypeString, Label: "Verbosity", Options: []core.ConnectionOption{
		{Name: "0 (Normal)", Value: "0"},
		{Name: "1 (Verbose)", Value: "1"},
		{Name: "2 (More Verbose)", Value: "2"},
		{Name: "3 (Debug)", Value: "3"},
		{Name: "4 (Connection Debug)", Value: "4"},
		{Name: "5 (WinRM Debug)", Value: "5"},
	}},
	{Name: "forks", Type: core.ConnectionTypeInteger, Label: "Forks", Placeholder: "How many hosts to work on at once — blank uses the AWX default"},
	{Name: "become_enabled", Type: core.ConnectionTypeBoolean, Label: "Run with sudo (become)", Placeholder: "Escalate to root on the target hosts"},
	{Name: "diff_mode", Type: core.ConnectionTypeBoolean, Label: "Show Changes (diff mode)", Placeholder: "Report what each host would change, line by line"},
	{Name: "extra_vars", Type: core.ConnectionTypeObject, Label: "Extra Variables", Placeholder: `{"target_dir":"/srv"} — AWX rejects any variable whose name starts with ansible_`},
	{Name: "execution_environment_id", Type: core.ConnectionTypeString, Label: "Execution Environment", Placeholder: "Blank uses the inventory's / AWX's default execution environment"},

	// --- Waiting ---
	{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Label: "Wait for Completion", Placeholder: "Hold the flow until the command finishes, then return its status and output"},
	{Name: "poll_interval_seconds", Type: core.ConnectionTypeInteger, Label: "Poll Interval (seconds)", Placeholder: "How often to check the command — default 5s", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "Give up waiting after this long — default 600s (max 3600). The command is NOT cancelled.", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "ignore_job_failure", Type: core.ConnectionTypeBoolean, Label: "Ignore Command Failure", Placeholder: "By default the node fails when the AWX command ends failed/error/canceled. Tick to succeed regardless.", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "include_stdout", Type: core.ConnectionTypeBoolean, Label: "Include Output", Placeholder: "Return the command's plain-text output (up to 1 MB)", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},

	// --- The guard. LAST and Required, on purpose. ---
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This runs a command on real hosts. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Ad-Hoc Command ID"},
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Kind"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "timed_out", Type: core.ConnectionTypeBoolean, Label: "Timed Out"},
	{Name: "host_status_counts", Type: core.ConnectionTypeObject, Label: "Host Results"},
	{Name: "stdout", Type: core.ConnectionTypeText, Label: "Output"},
	{Name: "elapsed", Type: core.ConnectionTypeString, Label: "Elapsed (seconds)"},
	{Name: "job_explanation", Type: core.ConnectionTypeString, Label: "Explanation"},
	{Name: "result_traceback", Type: core.ConnectionTypeString, Label: "Traceback"},
	{Name: "event_processing_finished", Type: core.ConnectionTypeBoolean, Label: "Events Written"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "AWX Link"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Ad-Hoc Command"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// The ONLY hard failure in this action: a missing or malformed credential is a
	// mis-configured node, not a failed request. Everything below soft-fails.
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// Guard first, before a single byte leaves the process.
	if err := awx.ConfirmDestructive(inputs, "run an ad-hoc command against the hosts in this inventory"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	inventoryID, err := awx.RequiredInt("inventory_id", "Inventory", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	credentialID, err := awx.RequiredInt("credential_id", "Machine Credential", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	moduleName := awx.OptionalString("module_name", inputs)
	if moduleName == "" {
		moduleName = defaultModule
	}
	moduleArgs := awx.OptionalString("module_args", inputs)
	if err := validateModule(moduleName, moduleArgs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"inventory":   inventoryID,
		"credential":  credentialID,
		"module_name": moduleName,
	}
	if moduleArgs != "" {
		body["module_args"] = moduleArgs
	}
	awx.SetIfPresent(body, inputs, "limit", "limit")
	awx.SetIfPresent(body, inputs, "job_type", "job_type")
	if err := awx.SetIntIfPresent(body, inputs, "verbosity", "verbosity"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.SetIntIfPresent(body, inputs, "forks", "forks"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	// Tri-state: AWX defaults both to false, but an untouched checkbox still means
	// "omit" rather than "send false" — SetBoolIfSet keeps the two apart.
	awx.SetBoolIfSet(body, inputs, "become_enabled", "become_enabled")
	awx.SetBoolIfSet(body, inputs, "diff_mode", "diff_mode")
	if err := awx.SetIntIfPresent(body, inputs, "execution_environment", "execution_environment_id"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	extraVars, err := awx.OptionalJSONObject("extra_vars", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := validateExtraVars(extraVars); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if len(extraVars) > 0 {
		body["extra_vars"] = extraVars
	}

	ctx, cancel := awx.Context()
	defer cancel()

	created, err := awx.CreateResource(ctx, auth, "ad_hoc_commands/", body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	id, _, err := awx.LaunchedJob(created, awx.JobKindAdHocCommand)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	target := fmt.Sprintf("%s on the hosts of inventory %d", moduleName, inventoryID)
	job := created
	timedOut := false
	stdout := ""

	if awx.BoolInput("wait_for_completion", inputs) {
		poll, _ := awx.OptionalInt("poll_interval_seconds", inputs)
		timeout, _ := awx.OptionalInt("timeout_seconds", inputs)
		timeout = awx.ClampWaitSeconds(timeout)
		includeStdout := awx.BoolInput("include_stdout", inputs)

		// WaitForJob deliberately does not inherit ctx's 75-second deadline — see
		// its doc comment — so the operator's timeout is the one that applies.
		res, err := awx.WaitForJob(ctx, auth, awx.JobKindAdHocCommand, id, awx.WaitOpts{
			PollIntervalSeconds: poll,
			TimeoutSeconds:      timeout,
			IncludeStdout:       includeStdout,
			WaitForEvents:       includeStdout,
		})
		if err != nil {
			return awx.ErrorResult(err.Error()), nil
		}
		job = res.Job
		timedOut = res.TimedOut
		stdout = res.Stdout

		if timedOut {
			// Emit the FULL output map, not an ErrorResult: the command is still
			// running, and the flow needs its ID to wait on it or cancel it.
			return failure(auth, job, id, stdout, true, fmt.Sprintf(
				"The ad-hoc command is still running after %ds. It has NOT been cancelled — ad-hoc command %d is still going in AWX (%s). Use Wait for Job or Cancel Job on that ID.",
				timeout, id, awx.JobURL(auth, awx.JobKindAdHocCommand, id))), nil
		}
		if !awx.BoolInput("ignore_job_failure", inputs) && jobFailed(job) {
			return failure(auth, job, id, stdout, false, fmt.Sprintf(
				"The ad-hoc command finished %s (%s ran against inventory %d). See %s%s",
				awx.StringField(job, "status"), moduleName, inventoryID,
				awx.JobURL(auth, awx.JobKindAdHocCommand, id), explanation(job))), nil
		}
	} else {
		// ★ STALE STATUS. The 201 was serialised before AWX signalled the command
		// to start, so its status is a stale "new". Re-read the record so the flow
		// sees what AWX actually thinks. A failure here is not fatal — the command
		// IS running — so the 201 is kept as a fallback.
		if detail, err := awx.GetResource(ctx, auth, fmt.Sprintf("ad_hoc_commands/%d/", id), nil); err == nil {
			job = detail
		}
	}

	out := outputs(auth, job, stdout, timedOut)
	out["tool_result"] = fmt.Sprintf("Ad-hoc command %d (%s) is %s", id, target, statusOrStarted(job))
	return out, nil
}

// outputs builds the standard job-shaped output map for an ad-hoc command.
// Ad-hoc commands have NO artifacts, so JobOutputs' key is dropped rather than
// emitted as an always-empty object.
func outputs(auth awx.Auth, job map[string]interface{}, stdout string, timedOut bool) map[string]interface{} {
	out := awx.JobOutputs(auth, awx.JobKindAdHocCommand, job)
	delete(out, "artifacts")
	out["stdout"] = stdout
	out["timed_out"] = timedOut
	out["tool_result"] = ""
	out["success"] = true
	out["error"] = ""
	return out
}

// failure returns the full output map with the failure flags set, rather than an
// ErrorResult — the command exists in AWX and the flow must still be able to see
// its ID, status and output. success=false with a nil Go error is the SOFT
// failure the flow engine wants: the node is marked failed, the flow keeps going.
func failure(auth awx.Auth, job map[string]interface{}, id int64, stdout string, timedOut bool, msg string) map[string]interface{} {
	out := outputs(auth, job, stdout, timedOut)
	out["job_id"] = awx.IDString(id) // the record may be sparse if AWX went quiet
	out["success"] = false
	out["error"] = msg
	out["tool_result"] = msg
	return out
}

// jobFailed reports an unsuccessful outcome. `failed` is AWX's own flag; the
// status check catches a canceled command, which is not a "failure" in AWX's
// bookkeeping but is certainly not a success for a flow.
func jobFailed(job map[string]interface{}) bool {
	if awx.BoolField(job, "failed") {
		return true
	}
	switch awx.StringField(job, "status") {
	case "failed", "error", "canceled":
		return true
	}
	return false
}

// explanation appends whatever AWX said about why the command did not run, when
// it said anything at all.
func explanation(job map[string]interface{}) string {
	if why := awx.StringField(job, "job_explanation"); why != "" {
		return " — " + why
	}
	if tb := awx.StringField(job, "result_traceback"); tb != "" {
		return " — " + strings.SplitN(strings.TrimSpace(tb), "\n", 2)[0]
	}
	return ""
}

func statusOrStarted(job map[string]interface{}) string {
	if s := awx.StringField(job, "status"); s != "" {
		return s
	}
	return "started"
}

// validateModule catches the two module mistakes AWX answers with an opaque 400.
//
// It deliberately does NOT check module_name against a hardcoded allow-list:
// AD_HOC_COMMANDS is an admin-editable runtime setting, so an instance may
// legitimately permit modules this node has never heard of, and a client-side
// allow-list would refuse them. The editor's live dropdown reads the instance's
// real list instead. What IS always wrong, on every instance, is a fully-qualified
// collection name — AWX matches the setting by SHORT NAME only.
func validateModule(moduleName, moduleArgs string) error {
	if strings.Contains(moduleName, ".") {
		short := moduleName[strings.LastIndex(moduleName, ".")+1:]
		return fmt.Errorf("AWX takes short module names only, so %q will be refused — use %q instead", moduleName, short)
	}
	if argsRequiredModules[moduleName] && moduleArgs == "" {
		return fmt.Errorf("Module Arguments is required for the %s module — it is the command to run, e.g. \"systemctl restart nginx\"", moduleName)
	}
	return nil
}

// validateExtraVars refuses the ansible_* variables AWX rejects outright, naming
// them, rather than letting the operator decode a 400.
func validateExtraVars(vars map[string]interface{}) error {
	reserved := []string{}
	for key := range vars {
		if strings.HasPrefix(key, "ansible_") {
			reserved = append(reserved, key)
		}
	}
	if len(reserved) == 0 {
		return nil
	}
	sort.Strings(reserved) // map order is random; the message must not be
	return fmt.Errorf("AWX will not accept an ad-hoc command whose Extra Variables set an ansible_ variable (%s). Remove it — connection settings belong on the machine credential or the inventory, not here.",
		strings.Join(reserved, ", "))
}
