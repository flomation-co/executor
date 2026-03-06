package actions

import (
	core "flomation.app/automate/executor"
	arithmetic_addition "flomation.app/automate/executor/actions/arithmetic/addition"
	arithmetic_division "flomation.app/automate/executor/actions/arithmetic/division"
	arithmetic_multiplication "flomation.app/automate/executor/actions/arithmetic/multiplication"
	arithmetic_subtraction "flomation.app/automate/executor/actions/arithmetic/subtraction"
	aws_ec2_describe "flomation.app/automate/executor/actions/aws/ec2/describe"
	aws_s3_delete "flomation.app/automate/executor/actions/aws/s3/delete"
	aws_s3_get "flomation.app/automate/executor/actions/aws/s3/get"
	aws_s3_list_bucket "flomation.app/automate/executor/actions/aws/s3/list"
	aws_s3_put "flomation.app/automate/executor/actions/aws/s3/put"
	"flomation.app/automate/executor/actions/common/smtp"
	output "flomation.app/automate/executor/actions/output/set"
	sql_query "flomation.app/automate/executor/actions/sql/query"
	"flomation.app/automate/executor/actions/trigger/manual"
)

var (
	Actions = map[string]core.Action{
		"aws/ec2/describe":          aws_ec2_describe.Execute,
		"aws/s3/delete":             aws_s3_delete.Execute,
		"aws/s3/get":                aws_s3_get.Execute,
		"aws/s3/put":                aws_s3_put.Execute,
		"aws/s3/list":               aws_s3_list_bucket.Execute,
		"trigger/manual":            manual.Execute,
		"output/set":                output.Execute,
		"common/smtp":               smtp.Execute,
		"sql/query":                 sql_query.Execute,
		"arithmetic/addition":       arithmetic_addition.Execute,
		"arithmetic/subtraction":    arithmetic_subtraction.Execute,
		"arithmetic/multiplication": arithmetic_multiplication.Execute,
		"arithmetic/division":       arithmetic_division.Execute,
	}
)
