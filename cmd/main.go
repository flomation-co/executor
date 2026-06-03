package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	contextFile := flag.String("context", "", "Path to execution context JSON file")
	checkpointFile := flag.String("checkpoint", "", "Path to checkpoint file for resumed execution")
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
				// The data might be a double-encoded JSON string — try unwrapping
				var s string
				if err2 := json.Unmarshal(tdBytes, &s); err2 == nil {
					if err3 := json.Unmarshal([]byte(s), &td); err3 == nil {
						flo.InjectTriggerData(td)
					} else {
						log.WithFields(log.Fields{"error": err3}).Warn("unable to parse unwrapped trigger data")
					}
				} else {
					log.WithFields(log.Fields{"error": err}).Warn("unable to parse trigger data")
				}
			} else {
				flo.InjectTriggerData(td)
			}
		}
	}

	// Load execution context for ${flow.xxx} variable substitution
	if *contextFile != "" {
		ctxBytes, err := os.ReadFile(*contextFile)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Warn("unable to read execution context file")
		} else {
			var ctx core.ExecutionContext
			if err := json.Unmarshal(ctxBytes, &ctx); err != nil {
				log.WithFields(log.Fields{"error": err}).Warn("unable to parse execution context")
			} else {
				if ctx.StartTime == "" {
					ctx.StartTime = time.Now().UTC().Format(time.RFC3339)
				}
				flo.SetContext(&ctx)

				// Configure mTLS client for internal API calls.
				if ctx.TLSCACert != "" && ctx.TLSCert != "" && ctx.TLSKey != "" {
					client, err := buildMTLSClient(ctx.TLSCACert, ctx.TLSCert, ctx.TLSKey)
					if err != nil {
						log.WithError(err).Warn("unable to configure mTLS — API calls will use plain HTTP")
					} else {
						ctx.APIClient = client
						flo.SetContext(&ctx) // re-set with client attached
						log.Info("mTLS configured for internal API calls")
					}
				}
			}
		}
	}

	// Restore checkpoint for resumed execution
	if *checkpointFile != "" {
		cpBytes, err := os.ReadFile(*checkpointFile)
		if err != nil {
			log.WithError(err).Error("unable to read checkpoint file")
			os.Exit(1)
		}
		var cp core.Checkpoint
		if err := json.Unmarshal(cpBytes, &cp); err != nil {
			log.WithError(err).Error("unable to parse checkpoint")
			os.Exit(1)
		}
		flo.RestoreCheckpoint(&cp)
		log.WithFields(log.Fields{
			"cached_nodes": len(cp.NodeResults),
			"variables":    len(cp.Variables),
		}).Info("restored checkpoint for resumed execution")
	}

	// Set up cancellable context with SIGTERM/SIGINT handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flo.SetCancelContext(ctx, cancel)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		log.WithFields(log.Fields{
			"signal": sig,
		}).Info("Received signal, cancelling execution")
		cancel()
	}()

	start := time.Now()
	status := int64(0)

	outputs, err := flo.Execute(actions.Actions, entryNode, e)
	if err != nil {
		if errors.Is(err, core.ErrSuspended) {
			log.Info("Execution suspended")
			status = 3
		} else {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Error executing flow")
			status = 1
			if errors.Is(err, core.ErrCancelled) {
				status = 2
			}
		}
	} else if flo.HadError() {
		// The error was handled by an On Error chain, but the execution
		// should still be marked as failed.
		status = 1
	}

	// Voice calls: treat WebSocket-close errors as normal termination.
	// The call ending is not a failure — it's the expected lifecycle.
	if status == 1 && flo.GetContext() != nil && flo.GetContext().ChannelType == "twilio_voice" {
		if err == nil || strings.Contains(err.Error(), "websocket") || strings.Contains(err.Error(), "WebSocket") || strings.Contains(err.Error(), "broken pipe") {
			status = 0
		}
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
		NodeResults:     flo.GetNodeExecutionResults(),
	}

	// Include checkpoint for suspended executions
	if status == 3 {
		result.Checkpoint = flo.CreateCheckpoint()
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

// buildMTLSClient creates an HTTP client that presents a client certificate
// and verifies the server against the platform CA. Used only for internal
// API calls — external calls (Anthropic, Google, etc.) use http.DefaultClient
// with the system CA pool.
func buildMTLSClient(caCertFile, certFile, keyFile string) (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load key pair: %w", err)
	}

	caCert, err := os.ReadFile(caCertFile) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      caPool,
				MinVersion:   tls.VersionTLS13,
			},
		},
	}, nil
}
