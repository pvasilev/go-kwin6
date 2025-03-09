SHELL=/bin/bash

.PHONY: build
build:
	GOOS=linux go build -o ./bin/kwin6-demo cmd/main.go
