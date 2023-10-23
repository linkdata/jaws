#!/bin/sh -e
go install golang.org/x/tools/cmd/stringer@latest
go generate ./...
go run github.com/axw/gocov/gocov@latest test -race -failfast -coverprofile=coverage.txt $* ./... | go run github.com/AlekSi/gocov-xml@latest > coverage.xml
go mod tidy
