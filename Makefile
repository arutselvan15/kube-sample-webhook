GO=${GO_CROSS_CMPL} GO111MODULE=on go
# for mac GO_CROSS_CMPL=GOOS=darwin GOARCH=amd64
# for linux GO_CROSS_CMPL=GOOS=linux GOARCH=amd64

NAME=estore-product-kube-webhook
BINARY=bin/${NAME}
MAIN_GO=cmd/main.go

BUILD=$(or ${BUILD_NUMBER},unknown)
VPREFIX=$(or ${VERSION_PERFIX}, v)
VERSION=${VPREFIX}.${BUILD}
DATE=$(shell date)
HOSTNAME=$(shell hostname)

mod-init:
	@echo "==> Mod Init..."
	${GO} mod init

all: clean deps fmt check test

clean:
	@echo "==> Cleaning..."
	rm -f report.json coverage.out

deps:
	@echo "==> Getting Dependencies..."
	${GO} mod tidy
	${GO} mod download

fmt:
	@echo "==> Code Formatting..."
	${GO} fmt ./...

check: fmt
	@echo "==> Code Check..."
	golangci-lint run --fast --tests

test: clean
	@echo "==> Testing..."
	CGO_ENABLED=0 ${GO} test -v -covermode=atomic -count=1 ./... -coverprofile coverage.out
	CGO_ENABLED=1 ${GO} test -race -covermode=atomic -count=1 ./... -json > report.json
	${GO} tool cover -func=coverage.out

gen-version:
	@echo "==> Generating Version..."
	echo "Version=${VERSION}" > version.txt
	echo "Date=${DATE}" >> version.txt
	echo "Host=${HOSTNAME}" >> version.txt
	cat version.txt

build: test gen-version
	@echo "==> Build Local..."
	CGO_ENABLED=0 ${GO} build -o ${BINARY} ${MAIN_GO}

container: build
	docker build -t ${NAME} .