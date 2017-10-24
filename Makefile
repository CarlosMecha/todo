test:
	@govendor test -race -cover +local

build:
	@govendor build -o bin/server cmd/server/main.go

run: test build
	bin/server --token=$(TOKEN) --key=todo-test.md

docker-build: build
	@docker build -t carlosmecha/todo:latest .

docker-run: docker-build
	@docker run --rm -e TOKEN=$(TOKEN)\
		-e AWS_SESSION_TOKEN=$(AWS_SESSION_TOKEN)\
		-e AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY)\
		-e AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID)\
		-e AWS_SECURITY_TOKEN=$(AWS_SECURITY_TOKEN)\
		--net host -p 6443:6443 carlosmecha/todo:latest -key=todo-test.md

.PHONY: test build run docker-build