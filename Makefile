test:
	go test ./...

cleantest:
	go clean -testcache && \
	go test ./...

lint:
	golangci-lint run --fix