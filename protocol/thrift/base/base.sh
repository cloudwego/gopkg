#! /bin/bash

set -e

thriftgo -g fastgo:no_default_serdes=true,gen_setter=true -o=.. ./base.thrift
gofmt -w base.go
gofmt -w k-base.go
