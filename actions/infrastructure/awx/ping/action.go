// Package infrastructure_awx_ping is the connection test — the action an operator
// runs first when something is wrong.
//
// It answers three separate questions, and keeps them separate:
//
//  1. CAN WE REACH AWX? On upstream AWX, GET {root}ping/ is AllowAny with
//     authentication_classes = () — no credential required. So it answers 200 even
//     when the token is garbage, which is exactly what lets this action tell "I
//     cannot reach your controller" apart from "your controller is there and it
//     rejected your credential". It also means a 200 from ping/ PROVES NOTHING
//     ABOUT THE CREDENTIAL, and this action must never let it be read that way.
//
//  2. WHERE IS THE API? The api_root output is the single most useful diagnostic
//     when an AAP 2.5 gateway is in play: upstream AWX and AAP ≤ 2.4 serve the
//     controller at /api/v2/, while AAP 2.5+ behind the platform gateway serves it
//     at /api/controller/v2/. This action echoes the root it actually detected.
//
//  3. DOES THE CREDENTIAL WORK? Answered by the authenticated GET {root}me/, which
//     requires auth on every deployment. credential_valid is that answer, and it —
//     not reachability — is what decides success.
package infrastructure_awx_ping

import (
	"fmt"

	core "flomation.app/automate/executor"
	awx "flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "AWX: Ping"
	Description  = "Check that Flomation can reach your AWX / AAP controller, and report its version. Use this to test a new connection."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+check"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// ---- AUTH BLOCK (identical on every AWX action; see awx.AuthInputs) ----
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
}

var Outputs = [...]core.Connection{
	{Name: "reachable", Type: core.ConnectionTypeBoolean, Label: "Reachable"},
	{Name: "credential_valid", Type: core.ConnectionTypeBoolean, Label: "Credential Accepted"},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Signed In As"},
	{Name: "version", Type: core.ConnectionTypeString, Label: "Version"},
	{Name: "product", Type: core.ConnectionTypeString, Label: "Product"},
	{Name: "api_root", Type: core.ConnectionTypeString, Label: "Detected API Root"},
	{Name: "ha", Type: core.ConnectionTypeBoolean, Label: "High Availability"},
	{Name: "active_node", Type: core.ConnectionTypeString, Label: "Active Node"},
	{Name: "instances", Type: core.ConnectionTypeObject, Label: "Instances"},
	{Name: "instance_groups", Type: core.ConnectionTypeObject, Label: "Instance Groups"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Ping Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err // the ONLY hard failure: the node is mis-configured
	}

	ctx, cancel := awx.Context()
	defer cancel()

	// ---- 1 + 2. REACHABILITY AND THE API ROOT -------------------------------
	root, rootErr := awx.ResolveAPIRoot(ctx, auth)
	prefix := root.Prefix

	var ping map[string]interface{}
	var reachErr error

	if rootErr == nil {
		ping, reachErr = awx.GetResource(ctx, auth, "ping/", nil)
	} else {
		// Discovery failed for one of two very different reasons: the credential was
		// rejected (AWX is right there and working), or we never found the API at
		// all. ping/ needs no credential, so probing it at each candidate root tells
		// them apart. Setting APIPrefix on a copy of the credential is what skips the
		// authenticated confirm step inside ResolveAPIRoot.
		reachErr = rootErr
		for _, candidate := range awx.CandidateRoots {
			probe := auth
			probe.APIPrefix = candidate
			obj, err := awx.GetResource(ctx, probe, "ping/", nil)
			if err == nil {
				prefix, ping, reachErr = candidate, obj, nil
				break
			}
		}
	}

	reachable := rootErr == nil || ping != nil

	// ---- 3. THE CREDENTIAL --------------------------------------------------
	// ping/ says nothing about the credential, so ask me/, which needs one on every
	// deployment. Pinning the prefix we found means this is a pure credential test
	// rather than a second round of discovery.
	credentialValid := false
	username := ""
	var meErr error
	if prefix != "" {
		credAuth := auth
		credAuth.APIPrefix = prefix
		me, err := awx.FetchMe(ctx, credAuth)
		if err != nil {
			meErr = err
		} else {
			credentialValid = true
			username = awx.StringField(me, "username")
			if awx.BoolField(me, "is_superuser") {
				username += " (superuser)"
			}
		}
	}

	// ---- OUTPUTS ------------------------------------------------------------
	version := awx.StringField(ping, "version")
	if version == "" {
		version = root.Version // the X-API-Product-Version header
	}
	product := root.Product // the X-API-Product-Name header
	if product == "" {
		product = "AWX"
		if prefix == "/api/controller/v2/" {
			product = "Red Hat Ansible Automation Platform"
		}
	}
	if ping == nil {
		ping = map[string]interface{}{}
	}

	out := map[string]interface{}{
		"reachable":        reachable,
		"credential_valid": credentialValid,
		"username":         username,
		"version":          version,
		"product":          product,
		"api_root":         prefix,
		"ha":               awx.BoolField(ping, "ha"),
		"active_node":      awx.StringField(ping, "active_node"),
		"instances":        ping["instances"],
		"instance_groups":  ping["instance_groups"],
		"result":           ping,
	}

	switch {
	case !reachable:
		merge(out, awx.ErrorResult(fmt.Sprintf("Could not reach AWX / AAP. %s", reachErr.Error())))

	case !credentialValid:
		// The distinction the whole action exists for: AWX answered, but it will not
		// accept this credential. Say both halves out loud, because a 200 from ping/
		// would otherwise be read as "the connection works".
		detail := "the credential could not be checked"
		if meErr != nil {
			detail = meErr.Error()
		}
		merge(out, awx.ErrorResult(fmt.Sprintf(
			"Reached %s at %s (API root %s), but it did not accept the credential. %s "+
				"(AWX answers its ping endpoint without a credential, so reaching it does not mean the token works.)",
			product, awx.OptionalString("awx_url", inputs), prefix, detail)))

	default:
		summary := fmt.Sprintf("Reached %s", product)
		if version != "" {
			summary += " " + version
		}
		summary += fmt.Sprintf(" — API root %s, signed in as %s", prefix, username)
		if awx.BoolField(ping, "ha") {
			summary += ". This controller is running in high-availability mode."
		}
		merge(out, awx.SuccessResult(summary, nil))
	}

	return out, nil
}

// merge folds the standard success/error keys into the diagnostic outputs, so a
// failed ping still reports the version and the API root it managed to discover —
// which is the whole point of a diagnostic action.
func merge(dst, src map[string]interface{}) {
	for k, v := range src {
		dst[k] = v
	}
}
