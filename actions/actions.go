package actions

import (
	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/aws/ec2/describe"
	aws_s3_delete "flomation.app/automate/executor/actions/aws/s3/delete"
	aws_s3_get "flomation.app/automate/executor/actions/aws/s3/get"
	aws_s3_put "flomation.app/automate/executor/actions/aws/s3/put"
	"flomation.app/automate/executor/actions/common/smtp"
	output "flomation.app/automate/executor/actions/output/set"
	"flomation.app/automate/executor/actions/trigger/manual"
)

var (
	Actions = map[string]core.Action{
		"aws/ec2/describe": aws_ec2_describe.Execute,
		"aws/s3/delete":    aws_s3_delete.Execute,
		"aws/s3/get":       aws_s3_get.Execute,
		"aws/s3/put":       aws_s3_put.Execute,
		"trigger/manual":   manual.Execute,
		"output/set":       output.Execute,
		"common/smtp":      smtp.Execute,
	}
)
