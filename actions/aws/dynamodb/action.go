package aws_dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "DynamoDB Query"
	Description  = "Execute an operation against an AWS DynamoDB table"
	Website      = "https://www.flomation.co"
	Icon         = "table"
	Date         = "21/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "access_key",
		Type:        core.ConnectionTypeString,
		Label:       "AWS Access Key",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "secret_key",
		Type:        core.ConnectionTypeString,
		Label:       "AWS Secret Key",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "region",
		Type:        core.ConnectionTypeString,
		Label:       "Region",
		Placeholder: "eu-west-1",
		Required:    true,
	},
	{
		Name:        "table_name",
		Type:        core.ConnectionTypeString,
		Label:       "Table Name",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:     "operation",
		Type:     core.ConnectionTypeString,
		Label:    "Operation",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Get Item", Value: "get_item"},
			{Name: "Put Item", Value: "put_item"},
			{Name: "Query", Value: "query"},
			{Name: "Scan", Value: "scan"},
			{Name: "Delete Item", Value: "delete_item"},
		},
	},
	{
		Name:        "key",
		Type:        core.ConnectionTypeText,
		Label:       "Key",
		Placeholder: "{\"pk\": {\"S\": \"value\"}}",
		Required:    true,
	},
	{
		Name:        "data",
		Type:        core.ConnectionTypeText,
		Label:       "Data",
		Placeholder: "{\"attribute\": {\"S\": \"value\"}}",
	},
	{
		Name:        "filter_expression",
		Type:        core.ConnectionTypeString,
		Label:       "Filter Expression",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "results",
		Type:  core.ConnectionTypeObject,
		Label: "Results",
	},
	{
		Name:  "count",
		Type:  core.ConnectionTypeInteger,
		Label: "Count",
	},
}

func getClient(accessKey, secretKey, region string) (*dynamodb.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     accessKey,
				SecretAccessKey: secretKey,
				SessionToken:    "",
				Source:          "Flomation DynamoDB Service",
			},
		}))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return dynamodb.NewFromConfig(cfg), nil
}

func parseDynamoDBKey(raw string) (map[string]types.AttributeValue, error) {
	var jsonMap map[string]map[string]string
	if err := json.Unmarshal([]byte(raw), &jsonMap); err != nil {
		return nil, fmt.Errorf("invalid key JSON: %w", err)
	}

	result := make(map[string]types.AttributeValue)
	for k, v := range jsonMap {
		if s, ok := v["S"]; ok {
			result[k] = &types.AttributeValueMemberS{Value: s}
		} else if n, ok := v["N"]; ok {
			result[k] = &types.AttributeValueMemberN{Value: n}
		} else if b, ok := v["BOOL"]; ok {
			result[k] = &types.AttributeValueMemberBOOL{Value: b == "true"}
		}
	}
	return result, nil
}

func parseDynamoDBItem(raw string) (map[string]types.AttributeValue, error) {
	return parseDynamoDBKey(raw)
}

func unmarshalItem(item map[string]types.AttributeValue) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	if err := attributevalue.UnmarshalMap(item, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func unmarshalItems(items []map[string]types.AttributeValue) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		m, err := unmarshalItem(item)
		if err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, nil
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	accessKeyConn := core.FindConnection("access_key", inputs)
	secretKeyConn := core.FindConnection("secret_key", inputs)
	regionConn := core.FindConnection("region", inputs)
	tableConn := core.FindConnection("table_name", inputs)
	opConn := core.FindConnection("operation", inputs)
	keyConn := core.FindConnection("key", inputs)
	dataConn := core.FindConnection("data", inputs)
	filterConn := core.FindConnection("filter_expression", inputs)

	accessKey := *accessKeyConn.String()
	secretKey := *secretKeyConn.String()
	region := *regionConn.String()
	tableName := *tableConn.String()
	operation := *opConn.String()

	log.WithFields(log.Fields{
		"table":     tableName,
		"region":    region,
		"operation": operation,
	}).Info("Connecting to DynamoDB")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := getClient(accessKey, secretKey, region)
	if err != nil {
		return nil, err
	}

	switch operation {
	case "get_item":
		keyMap, err := parseDynamoDBKey(*keyConn.String())
		if err != nil {
			return nil, err
		}

		result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(tableName),
			Key:       keyMap,
		})
		if err != nil {
			return nil, fmt.Errorf("GetItem failed: %w", err)
		}

		if result.Item == nil {
			return map[string]interface{}{
				"results": nil,
				"count":   int64(0),
			}, nil
		}

		item, err := unmarshalItem(result.Item)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal item: %w", err)
		}

		return map[string]interface{}{
			"results": item,
			"count":   int64(1),
		}, nil

	case "put_item":
		if dataConn == nil || dataConn.String() == nil || *dataConn.String() == "" {
			return nil, fmt.Errorf("data is required for put_item")
		}

		item, err := parseDynamoDBItem(*dataConn.String())
		if err != nil {
			return nil, fmt.Errorf("invalid data: %w", err)
		}

		_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item:      item,
		})
		if err != nil {
			return nil, fmt.Errorf("PutItem failed: %w", err)
		}

		return map[string]interface{}{
			"results": map[string]interface{}{"success": true},
			"count":   int64(1),
		}, nil

	case "query":
		keyExpr := *keyConn.String()
		input := &dynamodb.QueryInput{
			TableName:              aws.String(tableName),
			KeyConditionExpression: aws.String(keyExpr),
		}

		if filterConn != nil && filterConn.String() != nil && *filterConn.String() != "" {
			input.FilterExpression = aws.String(*filterConn.String())
		}

		result, err := client.Query(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("Query failed: %w", err)
		}

		items, err := unmarshalItems(result.Items)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal items: %w", err)
		}

		return map[string]interface{}{
			"results": items,
			"count":   int64(result.Count),
		}, nil

	case "scan":
		input := &dynamodb.ScanInput{
			TableName: aws.String(tableName),
		}

		if filterConn != nil && filterConn.String() != nil && *filterConn.String() != "" {
			input.FilterExpression = aws.String(*filterConn.String())
		}

		result, err := client.Scan(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("Scan failed: %w", err)
		}

		items, err := unmarshalItems(result.Items)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal items: %w", err)
		}

		return map[string]interface{}{
			"results": items,
			"count":   int64(result.Count),
		}, nil

	case "delete_item":
		keyMap, err := parseDynamoDBKey(*keyConn.String())
		if err != nil {
			return nil, err
		}

		_, err = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(tableName),
			Key:       keyMap,
		})
		if err != nil {
			return nil, fmt.Errorf("DeleteItem failed: %w", err)
		}

		return map[string]interface{}{
			"results": map[string]interface{}{"success": true},
			"count":   int64(1),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}
