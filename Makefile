NAMESPACE			= flomation.app/automate/executor
DATE				= $(shell date -u +%Y%m%d_%H%M%S)
NAME				?= execute

BRANCH 				:= $(shell git rev-parse --abbrev-ref HEAD)
GITHASH 			?= $(shell git rev-parse HEAD)
CI_PIPELINE_ID 		?= local
VERSION 			?= 0.0.${CI_PIPELINE_ID}

build: lint
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -X $(NAMESPACE)/internal/version.Version=$(VERSION) -X $(NAMESPACE)/internal/version.Hash=$(GITHASH) -X $(NAMESPACE)/internal/version.BuiltDate=$(DATE)" -o ./dist/flomation-${NAME} $(NAMESPACE)/cmd
	cd dist && zip -r ../build.zip .

lint:
	goimports -l .
	golangci-lint run --timeout=5m ./...
	gosec ./...

test:
	go test ./... -coverprofile cover.out
	go tool cover -func cover.out

flow:
	go run cmd/main.go -path flows/email.flo
