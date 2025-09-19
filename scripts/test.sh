#!/bin/bash

set -e
go test $(go list {./pkg/...,./internal/...} | grep -v /vendor/) -v -coverprofile cover.out
go tool cover -func=cover.out | grep total