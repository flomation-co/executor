package main

import (
	"flag"
	"time"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions"
	"flomation.app/automate/executor/internal/version"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.DebugLevel)

	log.WithFields(log.Fields{
		"version": version.Version,
		"hash":    version.GetHash(),
		"date":    version.BuiltDate,
	}).Info("Starting Flomation Executor")

	path := flag.String("path", "", "Path of the Flow file to execute")
	entry := flag.String("entry", "", "Entry node to begin execution")
	id := flag.String("id", uuid.NewString(), "Execution ID")
	api := flag.String("api", "https://api.flomation.app", "URL for API Service")
	environment := flag.String("environment", "default", "ID of environment to execute within")
	runner := flag.String("runner", "local", "Runner ID")

	flag.Parse()
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
		"environment": *environment,
		"runner":      *runner,
	}).Info("Executing Flow")

	flo, err := core.Load(path)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to load flow")
		return
	}

	// TODO: Determine where to pass in Environment details, etc
	var entryNode *string
	if *entry != "" {
		entryNode = entry
	}

	start := time.Now()
	outputs, err := flo.Execute(actions.Actions, entryNode, environment)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error executing flow")
	}

	duration := time.Since(start)

	//b, _ := json.Marshal(outputs)
	log.WithFields(log.Fields{
		"outputs":     outputs,
		"duration_ms": duration.Milliseconds(),
	}).Info("finished processing Flow")
}
