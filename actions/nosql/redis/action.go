package nosql_redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	core "flomation.app/automate/executor"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Redis Command"
	Description  = "Execute a command against a Redis instance"
	Website      = "https://www.flomation.co"
	Icon         = "bolt"
	Date         = "21/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "host",
		Type:        core.ConnectionTypeString,
		Label:       "Host",
		Placeholder: "localhost",
		Required:    true,
	},
	{
		Name:        "port",
		Type:        core.ConnectionTypeInteger,
		Label:       "Port",
		Placeholder: "6379",
		Required:    true,
	},
	{
		Name:        "password",
		Type:        core.ConnectionTypeString,
		Label:       "Password",
		Placeholder: "",
	},
	{
		Name:        "database",
		Type:        core.ConnectionTypeInteger,
		Label:       "Database",
		Placeholder: "0",
	},
	{
		Name:     "command",
		Type:     core.ConnectionTypeString,
		Label:    "Command",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "GET", Value: "GET"},
			{Name: "SET", Value: "SET"},
			{Name: "DEL", Value: "DEL"},
			{Name: "HGET", Value: "HGET"},
			{Name: "HSET", Value: "HSET"},
			{Name: "LPUSH", Value: "LPUSH"},
			{Name: "LPOP", Value: "LPOP"},
			{Name: "RPUSH", Value: "RPUSH"},
			{Name: "RPOP", Value: "RPOP"},
			{Name: "KEYS", Value: "KEYS"},
			{Name: "EXISTS", Value: "EXISTS"},
			{Name: "EXPIRE", Value: "EXPIRE"},
			{Name: "TTL", Value: "TTL"},
		},
	},
	{
		Name:        "key",
		Type:        core.ConnectionTypeString,
		Label:       "Key",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "value",
		Type:        core.ConnectionTypeText,
		Label:       "Value",
		Placeholder: "",
	},
	{
		Name:        "field",
		Type:        core.ConnectionTypeString,
		Label:       "Field",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "result",
		Type:  core.ConnectionTypeObject,
		Label: "Result",
	},
	{
		Name:  "success",
		Type:  core.ConnectionTypeBoolean,
		Label: "Success",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	hostConn := core.FindConnection("host", inputs)
	portConn := core.FindConnection("port", inputs)
	passwordConn := core.FindConnection("password", inputs)
	dbConn := core.FindConnection("database", inputs)
	cmdConn := core.FindConnection("command", inputs)
	keyConn := core.FindConnection("key", inputs)
	valueConn := core.FindConnection("value", inputs)
	fieldConn := core.FindConnection("field", inputs)

	host := *hostConn.String()
	port := *portConn.Number()
	command := *cmdConn.String()
	key := *keyConn.String()

	password := ""
	if passwordConn != nil && passwordConn.Value != nil && passwordConn.String() != nil {
		password = *passwordConn.String()
	}

	db := 0
	if dbConn != nil && dbConn.Value != nil && dbConn.Number() != nil {
		db = int(*dbConn.Number())
	}

	log.WithFields(log.Fields{
		"host":    host,
		"port":    port,
		"command": command,
		"key":     key,
	}).Info("Connecting to Redis")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       db,
	})
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	getValue := func() string {
		if valueConn != nil && valueConn.String() != nil {
			return *valueConn.String()
		}
		return ""
	}

	getField := func() string {
		if fieldConn != nil && fieldConn.String() != nil {
			return *fieldConn.String()
		}
		return ""
	}

	switch command {
	case "GET":
		val, err := client.Get(ctx, key).Result()
		if err == redis.Nil {
			return map[string]interface{}{"result": nil, "success": true}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("GET failed: %w", err)
		}
		return map[string]interface{}{"result": val, "success": true}, nil

	case "SET":
		err := client.Set(ctx, key, getValue(), 0).Err()
		if err != nil {
			return nil, fmt.Errorf("SET failed: %w", err)
		}
		return map[string]interface{}{"result": "OK", "success": true}, nil

	case "DEL":
		count, err := client.Del(ctx, key).Result()
		if err != nil {
			return nil, fmt.Errorf("DEL failed: %w", err)
		}
		return map[string]interface{}{"result": count, "success": true}, nil

	case "HGET":
		val, err := client.HGet(ctx, key, getField()).Result()
		if err == redis.Nil {
			return map[string]interface{}{"result": nil, "success": true}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("HGET failed: %w", err)
		}
		return map[string]interface{}{"result": val, "success": true}, nil

	case "HSET":
		err := client.HSet(ctx, key, getField(), getValue()).Err()
		if err != nil {
			return nil, fmt.Errorf("HSET failed: %w", err)
		}
		return map[string]interface{}{"result": "OK", "success": true}, nil

	case "LPUSH":
		count, err := client.LPush(ctx, key, getValue()).Result()
		if err != nil {
			return nil, fmt.Errorf("LPUSH failed: %w", err)
		}
		return map[string]interface{}{"result": count, "success": true}, nil

	case "LPOP":
		val, err := client.LPop(ctx, key).Result()
		if err == redis.Nil {
			return map[string]interface{}{"result": nil, "success": true}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("LPOP failed: %w", err)
		}
		return map[string]interface{}{"result": val, "success": true}, nil

	case "RPUSH":
		count, err := client.RPush(ctx, key, getValue()).Result()
		if err != nil {
			return nil, fmt.Errorf("RPUSH failed: %w", err)
		}
		return map[string]interface{}{"result": count, "success": true}, nil

	case "RPOP":
		val, err := client.RPop(ctx, key).Result()
		if err == redis.Nil {
			return map[string]interface{}{"result": nil, "success": true}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("RPOP failed: %w", err)
		}
		return map[string]interface{}{"result": val, "success": true}, nil

	case "KEYS":
		keys, err := client.Keys(ctx, key).Result()
		if err != nil {
			return nil, fmt.Errorf("KEYS failed: %w", err)
		}
		return map[string]interface{}{"result": keys, "success": true}, nil

	case "EXISTS":
		count, err := client.Exists(ctx, key).Result()
		if err != nil {
			return nil, fmt.Errorf("EXISTS failed: %w", err)
		}
		return map[string]interface{}{"result": count > 0, "success": true}, nil

	case "EXPIRE":
		seconds, err := strconv.ParseInt(getValue(), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("EXPIRE requires a numeric value (seconds): %w", err)
		}
		ok, err := client.Expire(ctx, key, time.Duration(seconds)*time.Second).Result()
		if err != nil {
			return nil, fmt.Errorf("EXPIRE failed: %w", err)
		}
		return map[string]interface{}{"result": ok, "success": true}, nil

	case "TTL":
		ttl, err := client.TTL(ctx, key).Result()
		if err != nil {
			return nil, fmt.Errorf("TTL failed: %w", err)
		}
		return map[string]interface{}{"result": int64(ttl.Seconds()), "success": true}, nil

	default:
		return nil, fmt.Errorf("unsupported command: %s", command)
	}
}
