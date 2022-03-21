# Shipyard

<!-- markdownlint-disable line-length -->
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/4865/badge)](https://bestpractices.coreinfrastructure.org/projects/4865)
[![Release Images](https://github.com/submariner-io/shipyard/workflows/Release%20Images/badge.svg)](https://github.com/submariner-io/shipyard/actions?query=workflow%3A%22Release+Images%22)
[![Periodic](https://github.com/submariner-io/shipyard/workflows/Periodic/badge.svg)](https://github.com/submariner-io/shipyard/actions?query=workflow%3APeriodic)
<!-- markdownlint-enable line-length -->

The Shipyard project provides tooling for creating K8s clusters with [kind](K8s in Docker) and provides a Go framework for creating E2E
tests.

## Prerequisites

- [go 1.12] with [$GOPATH configured]
- [docker]

## Usage

To use Shipyard for your project, it's easiest to use Dapper and Make.
To use Dapper, you'll need a specific Dockerfile that Dapper consumes to create a consistent environment based upon Shipyard's base image.
To use Make, you'll need some commands to enable Dapper and also include the targets which ship in the base image.

### Dockerfile.dapper

Shipyard provides this file automatically for you. You can also define it explicitly to be more tailored to the specific project.

The Dockerfile should build upon `quay.io/submariner/shipyard-dapper-base`.

For example, this very basic file allows E2E testing:

```Dockerfile
FROM quay.io/submariner/shipyard-dapper-base:feature-multi-active-gw

ENV DAPPER_SOURCE=/go/src/github.com/submariner-io/submariner DAPPER_DOCKER_SOCKET=true
ENV DAPPER_OUTPUT=${DAPPER_SOURCE}/output

WORKDIR ${DAPPER_SOURCE}

ENTRYPOINT ["./scripts/entry"]
CMD ["ci"]
```

You can also refer to the project's own [Dockerfile.dapper](Dockerfile.dapper) as an example.

### Makefile

The Makefile should include targets to run everything in Dapper.
They're defined in [Makefile.dapper](Makefile.dapper) and can be copied as-is and included, but it's best to download and import it.
To use Shipyard's target, simply include the [Makefile.inc](Makefile.inc) file in your own Makefile.

The simplest Makefile would look like this:

```Makefile
BASE_BRANCH=feature-multi-active-gw
PROJECT=shipyard
export BASE_BRANCH
export PROJECT

ifneq (,$(DAPPER_HOST_ARCH))

# Running in Dapper

include $(SHIPYARD_DIR)/Makefile.inc

else

# Not running in Dapper

Makefile.dapper:
        @echo Downloading $@
        @curl -sfLO https://raw.githubusercontent.com/submariner-io/shipyard/$(BASE_BRANCH)/$@

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
