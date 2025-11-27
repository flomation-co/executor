package actions

import (
	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/common/smtp"
	output "flomation.app/automate/executor/actions/output/set"
	"flomation.app/automate/executor/actions/trigger/manual"
)

var (
	Actions = map[string]core.Action{
		"manual":        manual.Execute,
		"output":        output.Execute,
		"sendSMTPEmail": smtp.Execute,
	}
)
