package main

import (
	"encoding/json"
	"flag"
	"flomation.app/automate/executor/internal/assets"
	"fmt"
	"os"
	"strings"
	"time"

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
	identity := flag.String("identity", "https://id.flomation.app", "URL of Identity Service Provider")
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
		return
	}

	log.WithFields(log.Fields{
		"path":        *path,
		"entry":       *entry,
		"id":          *id,
		"api":         *api,
		"environment": *env,
		"runner":      *runner,
	}).Info("Executing Flow")

	flo, err := core.Load(path)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Unable to load flow")
		return
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

		e, err = environment.NewEnvironment(*env, api, auth)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Unable to configure environment")
			return
		}
	}

	var entryNode *string
	if *entry != "" {
		entryNode = entry
	}

	start := time.Now()
	outputs, err := flo.Execute(actions.Actions, entryNode, e)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Error executing flow")
	}

	status := int64(0)
	if err != nil {
		status = 1
	}

	duration := time.Since(start)

	log.WithFields(log.Fields{
		"duration_ms": duration.Milliseconds(),
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
