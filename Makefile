test:
	@govendor test -race -cover +local

build:
	@govendor build -o bin/server cmd/server/main.go

run: test build
	bin/server --token=12345 --key=todo-test.md

.PHONY: test build run