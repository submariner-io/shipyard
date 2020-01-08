PROJECTNAME := armada
VERSION := $(shell git describe --tags | tr -d "v")
BUILD := $(shell git rev-parse HEAD)
USER := $(shell id -u)
OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
export GO111MODULE := on
export GOPROXY = https://proxy.golang.org

ifndef VERSION
override VERSION = dev
endif

# Go related variables.
GOCMD := go
GOBIN := $(shell go env GOPATH)/bin
GOBASE := $(shell pwd)
OUTPUTDIR := bin
GOLANGCILINT := $(GOBIN)/golangci-lint
PACKR := $(GOBIN)/packr2
GOIMPORTS := $(GOBIN)/goimports
GINKGO := $(GOBIN)/ginkgo

# # Use linker flags to provide version/build settings
LDFLAGS=-ldflags "-X github.com/submariner-io/armada/cmd/armada.Version=$(VERSION) -X github.com/submariner-io/armada/cmd/armada.Build=$(BUILD)"

$(GOLANGCILINT):
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) v1.17.0

$(PACKR):
	curl -sL https://github.com/gobuffalo/packr/releases/download/v2.7.1/packr_2.7.1_$(OS)_amd64.tar.gz | tar xzvf - packr2
	mv $(GOBASE)/packr2 $(GOBIN)/packr2
	chmod a+x $(GOBIN)/packr2

$(GOIMPORTS):
	GO111MODULE=off $(GOCMD) get -u golang.org/x/tools/cmd/goimports

$(GINKGO):
	GO111MODULE=off $(GOCMD) get -u github.com/onsi/ginkgo/ginkgo

test: $(GINKGO)
	ginkgo -v -cover ./pkg/...

e2e: $(GINKGO)
	ginkgo -v ./test/e2e/...

validate: $(GOLANGCILINT) $(GOIMPORTS)
	$(GOCMD) mod vendor
	find . -name '*.go' -not -wholename './vendor/*' | while read -r file; do goimports -w -d "$$file"; done
	golangci-lint run ./...

build: $(PACKR) validate
	packr2 -v --ignore-imports
	CGO_ENABLED=0 $(GOCMD) build $(LDFLAGS) -o $(GOBASE)/$(OUTPUTDIR)/$(PROJECTNAME)

docker-build-image:
	docker build -t $(PROJECTNAME):$(VERSION) --build-arg PROJECTNAME=$(PROJECTNAME) --build-arg OUTPUTDIR=$(OUTPUTDIR) .
	docker create --name $(PROJECTNAME)-$(VERSION)-builder $(PROJECTNAME):$(VERSION) /bin/sh

docker-build: docker-build-image
	$(eval CID=$(shell docker ps -aqf "name=$(PROJECTNAME)-$(VERSION)-builder"))
	docker cp $(CID):/$(PROJECTNAME)/$(OUTPUTDIR) .
	docker rm -f $(CID)

docker-run:
	${MAKE} docker ARGS="${ARGS}" || { echo "failure!"; ${MAKE} fix-perm; exit 1; };

docker:
	docker run -it --rm --name $(PROJECTNAME)-$(VERSION)-runner -v /var/run/docker.sock:/var/run/docker.sock -v $(GOBASE):/$(PROJECTNAME) -w /$(PROJECTNAME) quay.io/submariner/dapper-base:latest ${ARGS}
	sudo chown -R $(USER):$(USER) $(GOBASE)

clean: fix-perm
	rm -rf packrd debug packr2 $(OUTPUTDIR) $(GOBASE)/vendor $(GOBASE)/cmd/armada/armada-packr.go $(GOBASE)/pkg/*/*.cover* $(GOBASE)/test/*/*.cover* $(GOBASE)/pkg/*/output
	-docker ps -qf status=exited | xargs docker rm -f
	-docker ps -qaf name=$(PROJECTNAME)- | xargs docker rm -f
	-docker images -qf dangling=true | xargs docker rmi -f
	-docker volume ls -qf dangling=true | xargs docker volume rm -f
	-docker rmi $(PROJECTNAME):$(VERSION)

fix-perm:
	sudo chown -R $(USER):$(USER) $(GOBASE)

.PHONY: fix-perm clean docker docker-run docker-build docker-build-image build validate e2e test
