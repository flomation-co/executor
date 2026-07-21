// Package aws_rds_create_integration creates a zero-ETL integration replicating
// an Aurora source into an Amazon Redshift target.
package aws_rds_create_integration

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
	Name         = "AWS RDS Create Integration"
	Description  = "Create a zero-ETL integration from an Aurora source to a Redshift target."
	Website      = "https://www.flomation.co"
	Icon         = "link+plus"
	Date         = "21/07/2026"
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
	{Name: "integration_name", Type: core.ConnectionTypeString, Label: "Integration Name", Placeholder: "my-zero-etl-integration", Required: true},
	{Name: "source_arn", Type: core.ConnectionTypeString, Label: "Source ARN (Aurora cluster)", Placeholder: "arn:aws:rds:eu-west-2:<account>:cluster:my-cluster", Required: true},
	{Name: "target_arn", Type: core.ConnectionTypeString, Label: "Target ARN (Redshift namespace)", Placeholder: "arn:aws:redshift-serverless:eu-west-2:<account>:namespace/<id>", Required: true},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID (optional)", Placeholder: "Leave blank to use an AWS owned key"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "integration", Type: core.ConnectionTypeObject, Label: "Integration"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("integration_name", inputs)
	sourceArn := awscommon.InputString("source_arn", inputs)
	targetArn := awscommon.InputString("target_arn", inputs)
	if name == "" || sourceArn == "" || targetArn == "" {
		return nil, fmt.Errorf("integration name, source ARN and target ARN are all required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateIntegrationInput{
		IntegrationName: aws.String(name),
		SourceArn:       aws.String(sourceArn),
		TargetArn:       aws.String(targetArn),
	}
	if kms := awscommon.InputString("kms_key_id", inputs); kms != "" {
		in.KMSKeyId = aws.String(kms)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateIntegration(ctx, in)
	if err != nil {
		return nil, err
	}

	integration := map[string]interface{}{
		"integration_name": aws.ToString(out.IntegrationName),
		"integration_arn":  aws.ToString(out.IntegrationArn),
		"source_arn":       aws.ToString(out.SourceArn),
		"target_arn":       aws.ToString(out.TargetArn),
		"status":           string(out.Status),
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Creating integration %q (status: %s)", name, string(out.Status)),
		"integration": integration,
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
