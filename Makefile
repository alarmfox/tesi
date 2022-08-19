all: build


.PHONY: build
build:
	@go build -o build/server cmd/server/main.go
	@go build -o build/client cmd/client/main.go