package nosql_mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "MongoDB Query"
	Description  = "Execute an operation against a MongoDB collection"
	Website      = "https://www.flomation.co"
	Icon         = "leaf"
	Date         = "21/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "connection_uri",
		Type:        core.ConnectionTypeString,
		Label:       "Connection URI",
		Placeholder: "mongodb://localhost:27017",
		Required:    true,
	},
	{
		Name:        "database",
		Type:        core.ConnectionTypeString,
		Label:       "Database",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "collection",
		Type:        core.ConnectionTypeString,
		Label:       "Collection",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:     "operation",
		Type:     core.ConnectionTypeString,
		Label:    "Operation",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Find", Value: "find"},
			{Name: "Insert One", Value: "insert_one"},
			{Name: "Insert Many", Value: "insert_many"},
			{Name: "Update", Value: "update"},
			{Name: "Delete", Value: "delete"},
			{Name: "Count", Value: "count"},
		},
	},
	{
		Name:        "filter",
		Type:        core.ConnectionTypeText,
		Label:       "Filter",
		Placeholder: "{\"key\": \"value\"}",
	},
	{
		Name:        "document",
		Type:        core.ConnectionTypeText,
		Label:       "Document",
		Placeholder: "{\"key\": \"value\"}",
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

func parseJSON(raw string) (bson.M, error) {
	var doc bson.M
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return doc, nil
}

func parseJSONArray(raw string) ([]interface{}, error) {
	var docs []interface{}
	if err := json.Unmarshal([]byte(raw), &docs); err != nil {
		return nil, fmt.Errorf("invalid JSON array: %w", err)
	}
	return docs, nil
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	uriConn := core.FindConnection("connection_uri", inputs)
	dbConn := core.FindConnection("database", inputs)
	collConn := core.FindConnection("collection", inputs)
	opConn := core.FindConnection("operation", inputs)
	filterConn := core.FindConnection("filter", inputs)
	docConn := core.FindConnection("document", inputs)

	uri := *uriConn.String()
	dbName := *dbConn.String()
	collName := *collConn.String()
	operation := *opConn.String()

	log.WithFields(log.Fields{
		"database":   dbName,
		"collection": collName,
		"operation":  operation,
	}).Info("Connecting to MongoDB")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	coll := client.Database(dbName).Collection(collName)

	// Parse filter if provided
	filter := bson.M{}
	if filterConn != nil && filterConn.String() != nil && *filterConn.String() != "" {
		filter, err = parseJSON(*filterConn.String())
		if err != nil {
			return nil, fmt.Errorf("invalid filter: %w", err)
		}
	}

	switch operation {
	case "find":
		cursor, err := coll.Find(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("find failed: %w", err)
		}
		defer cursor.Close(ctx)

		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			return nil, fmt.Errorf("failed to decode results: %w", err)
		}
		if results == nil {
			results = []bson.M{}
		}

		return map[string]interface{}{
			"results": results,
			"count":   int64(len(results)),
		}, nil

	case "insert_one":
		if docConn == nil || docConn.String() == nil || *docConn.String() == "" {
			return nil, fmt.Errorf("document is required for insert_one")
		}
		doc, err := parseJSON(*docConn.String())
		if err != nil {
			return nil, fmt.Errorf("invalid document: %w", err)
		}

		result, err := coll.InsertOne(ctx, doc)
		if err != nil {
			return nil, fmt.Errorf("insert_one failed: %w", err)
		}

		return map[string]interface{}{
			"results": map[string]interface{}{"inserted_id": result.InsertedID},
			"count":   int64(1),
		}, nil

	case "insert_many":
		if docConn == nil || docConn.String() == nil || *docConn.String() == "" {
			return nil, fmt.Errorf("document is required for insert_many (provide a JSON array)")
		}
		docs, err := parseJSONArray(*docConn.String())
		if err != nil {
			return nil, fmt.Errorf("invalid documents array: %w", err)
		}

		result, err := coll.InsertMany(ctx, docs)
		if err != nil {
			return nil, fmt.Errorf("insert_many failed: %w", err)
		}

		return map[string]interface{}{
			"results": map[string]interface{}{"inserted_ids": result.InsertedIDs},
			"count":   int64(len(result.InsertedIDs)),
		}, nil

	case "update":
		if docConn == nil || docConn.String() == nil || *docConn.String() == "" {
			return nil, fmt.Errorf("document is required for update")
		}
		update, err := parseJSON(*docConn.String())
		if err != nil {
			return nil, fmt.Errorf("invalid update document: %w", err)
		}

		result, err := coll.UpdateMany(ctx, filter, update)
		if err != nil {
			return nil, fmt.Errorf("update failed: %w", err)
		}

		return map[string]interface{}{
			"results": map[string]interface{}{
				"matched_count":  result.MatchedCount,
				"modified_count": result.ModifiedCount,
			},
			"count": result.ModifiedCount,
		}, nil

	case "delete":
		result, err := coll.DeleteMany(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("delete failed: %w", err)
		}

		return map[string]interface{}{
			"results": map[string]interface{}{"deleted_count": result.DeletedCount},
			"count":   result.DeletedCount,
		}, nil

	case "count":
		count, err := coll.CountDocuments(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("count failed: %w", err)
		}

		return map[string]interface{}{
			"results": map[string]interface{}{"count": count},
			"count":   count,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}
