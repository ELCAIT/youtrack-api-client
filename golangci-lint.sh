#!/bin/sh

set -e
set -x

golangci-lint run --config .golangci.yml
