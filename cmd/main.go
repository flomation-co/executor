package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"flomation.app/automate/executor/internal/assets"

	"flomation.app/automate/executor/internal/environment"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions"
	"flomation.app/automate/executor/internal/version"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	LogFormatDefault = "default"
	LogFormatJSON    = "json"
)

const (
	DefaultAPI              = "https://api.flomation.app"
	DefaultEnvironment      = "default"
	DefaultRunnerIdentifier = "local"
)

func main() {
	printVersion := flag.Bool("version", false, "Print version information")
	manifest := flag.String("manifest", "", "Path to save the manifest file to")
	path := flag.String("path", "", "Path of the Flow file to execute")
	entry := flag.String("entry", "", "Entry node to begin execution")
	id := flag.String("id", uuid.NewString(), "Execution ID")
	flow := flag.String("flow", "", "Flow ID")
	api := flag.String("api", DefaultAPI, "URL for API Service")
	env := flag.String("environment", DefaultEnvironment, "ID of environment to execute within")
	runner := flag.String("runner", DefaultRunnerIdentifier, "Runner ID")
	logOutput := flag.String("output", LogFormatDefault, "Log output format - default/json")
	user := flag.String("user", "", "Execution context username")
	password := flag.String("password", "", "Execution context username")
	token := flag.String("token", "", "Execution context credential token")
	key := flag.String("key", "", "Certificate file for signing requests")
	identity := flag.String("identity", "https://id.flomation.app", "URL of Identity Service Provider")
	triggerData := flag.String("trigger-data", "", "Path to trigger invocation data JSON file")
	debug := flag.Bool("debug", false, "Enable debug logging")

	flag.Parse()
	if strings.ToLower(*logOutput) == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}

	if *printVersion {
		fmt.Printf("%v", version.Version)
		return
	}

	log.WithFields(log.Fields{
		"version": version.Version,
		"hash":    version.GetHash(),
		"date":    version.BuiltDate,
		"runner":  runner,
	}).Info("Starting Flomation Executor")

	if *manifest != "" {
		b, err := assets.Manifest.ReadFile("manifest/manifest.json")
		if err != nil {
			log.Fatal(err)
		}

		if err := os.WriteFile(*manifest, b, 0600); err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("unable to write manifest file")
		}
		return
	}

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	if *path == "" {
		log.Error("no path specified")
		flag.PrintDefaults()
		os.Exit(1)
	}

	flo, err := core.Load(path)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Unable to load flow")
		os.Exit(1)
	}

	var e *environment.Environment
	if *env != DefaultEnvironment {
		var auth *environment.Credentials
		if *user != "" && *password != "" {
			auth = environment.Authenticate(*user, *password, identity)
		}

		if *token != "" {
			auth = environment.Token(*token)
		}

		if *key != "" {
			auth, err = environment.Key(*id, *key)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Error("unable to load key")
			}
		}

		e, err = environment.NewEnvironment(*env, api, *id, auth)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("unable to configure environment")
			os.Exit(1)
		}
	}

	var entryNode *string
	if *entry != "" {
		entryNode = entry
	}

	// Load and inject trigger invocation data into the trigger node
	if *triggerData != "" {
		tdBytes, err := os.ReadFile(*triggerData)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Warn("unable to read trigger data file")
		} else {
			var td map[string]interface{}
			if err := json.Unmarshal(tdBytes, &td); err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Warn("unable to parse trigger data")
			} else {
				flo.InjectTriggerData(td)
			}
		}
	}

	start := time.Now()
	status := int64(0)

	outputs, err := flo.Execute(actions.Actions, entryNode, e)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Error executing flow")
		status = 1
	}

	duration := time.Since(start)

	log.WithFields(log.Fields{
		"duration_ms": duration.Milliseconds(),
		"status":      status,
	}).Info("Finished processing Flow")

	result := core.ExecutionResult{
		ID:              *id,
		FlowID:          *flow,
		Status:          status,
		Duration:        duration.Milliseconds(),
		BillingDuration: duration.Milliseconds(),
		Outputs:         outputs,
	}

	b, err := json.Marshal(result)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to marshal json")
	}

	if err := os.WriteFile("state.json", b, 0600); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to write state file")
	}
}
