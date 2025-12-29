NAMESPACE			= flomation.app/automate/executor
DATE				= $(shell date -u +%Y%m%d_%H%M%S)
NAME				?= execute
BRANCH 				:= $(shell git rev-parse --abbrev-ref HEAD)
GITHASH 			?= $(shell git rev-parse HEAD)
CI_PIPELINE_ID 		?= local
VERSION 			?= 0.0.${CI_PIPELINE_ID}

TOKEN = eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJodHRwczovL2Rldi5mbG9tYXRpb24uYXBwIiwiZXhwIjoxNzY1OTkwMzQ1LCJndWVzdCI6ZmFsc2UsImlkIjoiZTlhZjBlMGEtZmJjNy00OWU1LTkwMTQtZDgwZWQ4ODc5YWQ2IiwiaXNzIjoiaHR0cHM6Ly9kZXYuZmxvbWF0aW9uLmFwcCIsIm5iZiI6MTc2NTIxMjc0NSwicm9sZXMiOiJHVUVTVCIsInN1YiI6ImU5YWYwZTBhLWZiYzctNDllNS05MDE0LWQ4MGVkODg3OWFkNiIsInZlcmlmaWVkIjp0cnVlfQ.CL1LqHVetbwGC780xnoB8nt7G936m52-00XFo3iaO5DRs9HrhvUjgHvIqnxdJ2irqdYogjPAOhj5nrZksIjS7g

OS_ARCHS := \
	linux/amd64 \
	linux/arm64 \
	linux/arm \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

build: lint
	rm -rf dist/
	@for platform in $(OS_ARCHS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		echo "Building for $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags "-s -X $(NAMESPACE)/internal/version.Version=$(VERSION) -X $(NAMESPACE)/internal/version.Hash=$(GITHASH) -X $(NAMESPACE)/internal/version.BuiltDate=$(DATE)" -o ./dist/flomation-${NAME}-$$arch-$$os-${VERSION} $(NAMESPACE)/cmd; \
	done
	cd dist && zip -r ../build.zip .

lint:
	goimports -l .
	golangci-lint run --timeout=5m ./...
	gosec ./...

test:
	go test ./... -coverprofile cover.out
	go tool cover -func cover.out

flow:
	go run cmd/main.go -path flows/email.flo -api http://localhost:8080 -environment ae-dev-shared -identity http://localhost:8081 -user ${FLO_USER} -password ${FLO_PASS}