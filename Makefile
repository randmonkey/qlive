QLIVE_IMAGE_TAG ?= $(shell date +%Y%m%d%H%M%S)

all: dep
	GODEBUG=netdns=go go install -v ./...

linux: dep
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -v ./...

docker-image: dep
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o docker_images/qlive-server/qlive-linux .
	docker build docker_images/qlive-server -t qlive-server:${QLIVE_IMAGE_TAG}
	rm docker_images/qlive-server/qlive-linux

dep:

gofmt-check:
	@test `find . -name "*.go" |  xargs gofmt -s -l -e | wc -l` -eq 0

govet-check:
	go list ./... | xargs go vet -composites=false

test:
    CGO_ENABLED=0 go list ./... | xargs go test -timeout=150s

test-coverage:
	CGO_ENABLED=0 go test ./... -v -cover -timeout=150s

before-commit: gofmt-check govet-check test