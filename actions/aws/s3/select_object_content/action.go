// Package aws_s3_select_object_content runs an S3 Select SQL query over a single
// S3 object using the streaming SelectObjectContent event-stream API.
package aws_s3_select_object_content

import (
	"bytes"
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Select Object Content"
	Description  = "Run a SQL query over a single S3 object (CSV/JSON/Parquet) and return matching records."
	Website      = "https://www.flomation.co"
	Icon         = "code+magnifying-glass"
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
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Bucket", Placeholder: "my-bucket", Required: true},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Object Key", Placeholder: "path/to/object.csv", Required: true},
	{Name: "expression", Type: core.ConnectionTypeString, Label: "SQL Expression", Placeholder: "SELECT * FROM S3Object s LIMIT 10", Required: true},
	{Name: "input_format", Type: core.ConnectionTypeString, Label: "Input Format", Required: true, Options: []core.ConnectionOption{
		{Name: "CSV", Value: "csv"},
		{Name: "JSON", Value: "json"},
		{Name: "Parquet", Value: "parquet"},
	}},
	{Name: "csv_file_header", Type: core.ConnectionTypeString, Label: "CSV File Header (CSV only)", Options: []core.ConnectionOption{
		{Name: "Use header names", Value: "USE"},
		{Name: "Ignore header line", Value: "IGNORE"},
		{Name: "No header", Value: "NONE"},
	}, Visible: &core.VisibleWhen{Field: "input_format", Values: []string{"csv"}}},
	{Name: "compression", Type: core.ConnectionTypeString, Label: "Compression", Options: []core.ConnectionOption{
		{Name: "None", Value: "NONE"},
		{Name: "GZIP", Value: "GZIP"},
		{Name: "BZIP2", Value: "BZIP2"},
	}},
	{Name: "output_format", Type: core.ConnectionTypeString, Label: "Output Format", Options: []core.ConnectionOption{
		{Name: "JSON", Value: "json"},
		{Name: "CSV", Value: "csv"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "records", Type: core.ConnectionTypeString, Label: "Records"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	key := awscommon.InputString("key", inputs)
	expression := awscommon.InputString("expression", inputs)
	if bucket == "" || key == "" {
		return nil, fmt.Errorf("bucket and key are required")
	}
	if expression == "" {
		return nil, fmt.Errorf("expression (SQL) is required")
	}

	inputFormat := awscommon.InputString("input_format", inputs)
	if inputFormat == "" {
		return nil, fmt.Errorf("input_format is required")
	}

	// Build the input serialization for the chosen object format.
	inputSer := &s3types.InputSerialization{}
	switch inputFormat {
	case "csv":
		header := awscommon.InputString("csv_file_header", inputs)
		if header == "" {
			header = "USE"
		}
		inputSer.CSV = &s3types.CSVInput{FileHeaderInfo: s3types.FileHeaderInfo(header)}
	case "json":
		inputSer.JSON = &s3types.JSONInput{Type: s3types.JSONTypeDocument}
	case "parquet":
		inputSer.Parquet = &s3types.ParquetInput{}
	default:
		return nil, fmt.Errorf("unsupported input_format %q", inputFormat)
	}

	if compression := awscommon.InputString("compression", inputs); compression != "" && compression != "NONE" {
		inputSer.CompressionType = s3types.CompressionType(compression)
	}

	// Build the output serialization.
	outputFormat := awscommon.InputString("output_format", inputs)
	if outputFormat == "" {
		outputFormat = "json"
	}
	outputSer := &s3types.OutputSerialization{}
	switch outputFormat {
	case "csv":
		outputSer.CSV = &s3types.CSVOutput{}
	case "json":
		outputSer.JSON = &s3types.JSONOutput{}
	default:
		return nil, fmt.Errorf("unsupported output_format %q", outputFormat)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	resp, err := client.SelectObjectContent(ctx, &awsS3.SelectObjectContentInput{
		Bucket:              aws.String(bucket),
		Key:                 aws.String(key),
		Expression:          aws.String(expression),
		ExpressionType:      s3types.ExpressionTypeSql,
		InputSerialization:  inputSer,
		OutputSerialization: outputSer,
	})
	if err != nil {
		return nil, err
	}

	stream := resp.GetStream()
	defer stream.Close()

	var buf bytes.Buffer
	for event := range stream.Events() {
		if r, ok := event.(*s3types.SelectObjectContentEventStreamMemberRecords); ok {
			buf.Write(r.Value.Payload)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}

	records := buf.String()
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("S3 Select on %s/%s returned %d bytes", bucket, key, buf.Len()),
		"records":     records,
	}, nil
}
