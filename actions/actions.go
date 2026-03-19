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
	git_add "flomation.app/automate/executor/actions/git/add"
	git_branch "flomation.app/automate/executor/actions/git/branch"
	git_checkout "flomation.app/automate/executor/actions/git/checkout"
	git_clone "flomation.app/automate/executor/actions/git/clone"
	git_commit "flomation.app/automate/executor/actions/git/commit"
	git_pull "flomation.app/automate/executor/actions/git/pull"
	git_push "flomation.app/automate/executor/actions/git/push"
	git_status "flomation.app/automate/executor/actions/git/status"
	git_tag "flomation.app/automate/executor/actions/git/tag"
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
		"git/clone":                 git_clone.Execute,
		"git/checkout":              git_checkout.Execute,
		"git/add":                   git_add.Execute,
		"git/commit":                git_commit.Execute,
		"git/push":                  git_push.Execute,
		"git/pull":                  git_pull.Execute,
		"git/branch":                git_branch.Execute,
		"git/tag":                   git_tag.Execute,
		"git/status":                git_status.Execute,
	}
)
