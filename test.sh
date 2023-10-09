#!/bin/sh -e
go install golang.org/x/tools/cmd/stringer@latest
go install github.com/axw/gocov/gocov@latest
go install github.com/AlekSi/gocov-xml@latest
go generate ./...
gocov test -race -failfast -coverprofile=coverage.txt $* ./... | gocov-xml > coverage.xml
go mod tidy
