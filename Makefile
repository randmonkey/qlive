all: dep
	GODEBUG=netdns=go go install -v ./...

linux: dep
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -v ./...

dep:

gofmt-check:
	@test `find . -name "*.go" |  xargs gofmt -s -l -e | wc -l` -eq 0

govet-check:
	go list ./... | xargs go vet -composites=false

test:
    CGO_ENABLED=0 go list ./... | xargs go test -timeout=150s
