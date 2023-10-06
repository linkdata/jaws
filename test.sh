#!/bin/sh -e
go install golang.org/x/tools/cmd/stringer@latest
go generate ./...
go get github.com/boumenot/gocover-cobertura
go test -race -failfast -coverprofile=coverage.txt -count 5 $* ./...
go run github.com/boumenot/gocover-cobertura < coverage.txt > coverage.xml
go mod tidy
