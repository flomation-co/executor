// Package aws_secretsmanager_remove_regions_from_replication removes replica regions from a secret in AWS Secrets Manager.
package aws_secretsmanager_remove_regions_from_replication

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Secrets Manager Remove Regions From Replication"
	Description  = "Remove one or more replica regions from a replicated secret."
	Website      = "https://www.flomation.co"
	Icon         = "lock+trash"
	Date         = "22/07/2026"
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
	{Name: "secret_id", Type: core.ConnectionTypeString, Label: "Secret ID or ARN", Placeholder: "prod/db/password", Required: true},
	{Name: "regions", Type: core.ConnectionTypeString, Label: "Regions (comma-separated)", Placeholder: "us-east-1,eu-central-1", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "Secret ARN"},
	{Name: "replication_status", Type: core.ConnectionTypeString, Label: "Replication Status (JSON)"},
}

type replicationEntry struct {
	Region        string `json:"region"`
	Status        string `json:"status"`
	StatusMessage string `json:"status_message"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	secretID := awscommon.InputString("secret_id", inputs)
	if secretID == "" {
		return nil, fmt.Errorf("secret id is required")
	}
	regionsRaw := awscommon.InputString("regions", inputs)
	if regionsRaw == "" {
		return nil, fmt.Errorf("regions are required")
	}

	var removeRegions []string
	for _, r := range strings.Split(regionsRaw, ",") {
		if r = strings.TrimSpace(r); r != "" {
			removeRegions = append(removeRegions, r)
		}
	}
	if len(removeRegions) == 0 {
		return nil, fmt.Errorf("regions are required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := secretsmanager.NewFromConfig(cfg)

	out, err := client.RemoveRegionsFromReplication(ctx, &secretsmanager.RemoveRegionsFromReplicationInput{
		SecretId:             aws.String(secretID),
		RemoveReplicaRegions: removeRegions,
	})
	if err != nil {
		return nil, err
	}

	status := make([]replicationEntry, 0, len(out.ReplicationStatus))
	for _, s := range out.ReplicationStatus {
		status = append(status, replicationEntry{
			Region:        aws.ToString(s.Region),
			Status:        string(s.Status),
			StatusMessage: aws.ToString(s.StatusMessage),
		})
	}
	statusData, err := json.Marshal(status)
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.ARN)
	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Removed %d region(s) from replication of %s", len(removeRegions), secretID),
		"arn":                arn,
		"replication_status": string(statusData),
	}, nil
}
