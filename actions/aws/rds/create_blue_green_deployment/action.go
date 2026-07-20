// Package aws_rds_create_blue_green_deployment creates an RDS blue/green
// deployment cloning a source DB instance or cluster into a green environment.
package aws_rds_create_blue_green_deployment

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Create Blue/Green Deployment"
	Description  = "Create a blue/green deployment cloning a source DB into a green environment."
	Website      = "https://www.flomation.co"
	Icon         = "code-branch+plus"
	Date         = "20/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{
		{Name: "Access Keys", Value: "keys"},
		{Name: "Assume Role (cross-account)", Value: "assume_role"},
		{Name: "Managed Role (Credential)", Value: "credential"},
	}},
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Role ARN to Assume", Placeholder: "arn:aws:iam::<your-account>:role/FlomationAccess", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "AWS Role Credential", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"credential"}}},
	{Name: "blue_green_deployment_name", Type: core.ConnectionTypeString, Label: "Blue/Green Deployment Name", Placeholder: "my-blue-green-deployment", Required: true},
	{Name: "source", Type: core.ConnectionTypeString, Label: "Source ARN", Placeholder: "arn:aws:rds:eu-west-2:...:db:my-database", Required: true},
	{Name: "target_engine_version", Type: core.ConnectionTypeString, Label: "Target Engine Version (optional)", Placeholder: "e.g. 15.4"},
	{Name: "target_db_parameter_group_name", Type: core.ConnectionTypeString, Label: "Target DB Parameter Group (optional)"},
	{Name: "target_db_cluster_parameter_group_name", Type: core.ConnectionTypeString, Label: "Target DB Cluster Parameter Group (optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "deployment", Type: core.ConnectionTypeObject, Label: "Blue/Green Deployment"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("blue_green_deployment_name", inputs)
	if name == "" {
		return nil, fmt.Errorf("blue/green deployment name is required")
	}
	source := awscommon.InputString("source", inputs)
	if source == "" {
		return nil, fmt.Errorf("source ARN is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateBlueGreenDeploymentInput{
		BlueGreenDeploymentName: aws.String(name),
		Source:                  aws.String(source),
	}
	if v := awscommon.InputString("target_engine_version", inputs); v != "" {
		in.TargetEngineVersion = aws.String(v)
	}
	if v := awscommon.InputString("target_db_parameter_group_name", inputs); v != "" {
		in.TargetDBParameterGroupName = aws.String(v)
	}
	if v := awscommon.InputString("target_db_cluster_parameter_group_name", inputs); v != "" {
		in.TargetDBClusterParameterGroupName = aws.String(v)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateBlueGreenDeployment(ctx, in)
	if err != nil {
		return nil, err
	}

	dep := out.BlueGreenDeployment
	deployment := map[string]interface{}{
		"blue_green_deployment_identifier": aws.ToString(dep.BlueGreenDeploymentIdentifier),
		"name":                             aws.ToString(dep.BlueGreenDeploymentName),
		"source":                           aws.ToString(dep.Source),
		"target":                           aws.ToString(dep.Target),
		"status":                           aws.ToString(dep.Status),
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created blue/green deployment %q (status: %s)", aws.ToString(dep.BlueGreenDeploymentIdentifier), aws.ToString(dep.Status)),
		"deployment":  deployment,
	}, nil
}

func buildTags(inputs []*core.Connection) []rdstypes.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []rdstypes.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, rdstypes.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
