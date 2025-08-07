#!/bin/bash

GOOS=linux CGO_ENABLED=1 go build -o dispeysController -a -gcflags=all="-l -B" -ldflags="-s -w" cmd/controller/main.go
