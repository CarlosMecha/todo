test:
	@govendor test -race -cover +local

build:
	@govendor build -o bin/server cmd/server/main.go

run: test build
	bin/server --token=$(TOKEN) --key=todo-test.md

docker-build: build
	@docker build -t carlosmecha/todo:latest .

docker-run: docker-build
	@docker run --rm -e TOKEN=$(TOKEN) -e AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) -e AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) --net host -p 6060:6060 carlosmecha/todo:latest -key=todo-test.m

.PHONY: test build run docker-build