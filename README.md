# Shipyard

The Shipyard project provides tooling for creating K8s clusters with [kind](K8s in Docker) and provides a Go framework for creating E2E tests. 

[![Build Status](https://travis-ci.com/submariner-io/shipyard.svg?branch=master)](https://travis-ci.com/submariner-io/shipyard)
[![Go Report Card](https://goreportcard.com/badge/github.com/submariner-io/shipyard)](https://goreportcard.com/report/github.com/submariner-io/shipyard)

## Prerequisites

- [go 1.12] with [$GOPATH configured]
- [docker]

## Usage
To use shipyard for your project, it's easiest to use dapper and make.
To use dapper you'll need a specific Dockerfile that dapper consumes to create a consistent environment based upon shipyard's base image.
To use make you'll need some commands to enable dapper and also include the targets which ship in the base image.

### Dockerfile.dapper

The dockerfile should build upon `quay.io/submariner/shipyard-dapper-base`.
For example:

```Dockerfile
FROM quay.io/submariner/shipyard-dapper-base

ENV DAPPER_ENV="REPO TAG QUAY_USERNAME QUAY_PASSWORD TRAVIS_COMMIT" \
    DAPPER_SOURCE=/go/src/github.com/submariner-io/submariner DAPPER_DOCKER_SOCKET=true
ENV DAPPER_OUTPUT=${DAPPER_SOURCE}/output

WORKDIR ${DAPPER_SOURCE}

ENTRYPOINT ["./scripts/entry"]
CMD ["ci"]
```

You can also refer to the project's own [Dockerfile.dapper](Dockerfile.dapper) as an example.

### Makefile

The makefile should include targets to run everything in dapper, they're defined in [Makefile.dapper](Makefile.dapper) and can be copied as is and included. To use shipyard's target simply include the [Makefile.inc](Makefile.inc) file in your own Makefile.

The simplest Makefile would look like this:

```Makefile
ifneq (,$(DAPPER_HOST_ARCH))

# Running in Dapper

include $(SHIPYARD_DIR)/Makefile.inc

else

# Not running in Dapper

include Makefile.dapper

endif

# Disable rebuilding Makefile
Makefile Makefile.dapper Makefile.inc: ;
```

You can also refer to the project's own [Makefile](Makefile) as an example.

## Releases

Get the latest version from the [Releases] page.

<!--links-->
[go 1.12]: https://blog.golang.org/go1.12
[docker]: https://docs.docker.com/install/
[$GOPATH configured]: https://github.com/golang/go/wiki/SettingGOPATH
[Releases]: https://github.com/submariner-io/shipyard/releases/
[kind]: https://github.com/kubernetes-sigs/kind
