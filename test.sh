#!/bin/sh -e
go get github.com/boumenot/gocover-cobertura
go test -race -failfast -coverprofile=coverage.txt -count 100 $* ./...
go run github.com/boumenot/gocover-cobertura < coverage.txt > coverage.xml
go mod tidy
