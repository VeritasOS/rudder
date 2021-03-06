APP=rudder
VERSION=latest
LDFLAGS=-ldflags "-X github.com/AcalephStorage/rudder/cmd.version=${VERSION}"
IMAGE_NAME=quay.io/acaleph/rudder

all: deps build

clean:
	@echo "--> cleaning..."
	@rm -rf build
	@rm -rf vendor
	@go clean ./...

prereq:
	@mkdir -p build/{bin,tar}
	@go get -u github.com/Masterminds/glide

deps: prereq
	@glide install

build: prereq
	@echo '--> building...'
	@go fmt ./...
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/bin/${APP} ${LDFLAGS} ./cmd

package:
	@echo '--> packaging...'
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -o build/bin/${APP} ${LDFLAGS} ./cmd
	@docker build -t ${IMAGE_NAME}:${VERSION} .

deploy: package
	@echo '--> deploying...'
	@docker push quay.io/acaleph/rudder:${VERSION}
