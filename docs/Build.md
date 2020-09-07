# Build

## Prerequisites

- Just to build
  - `go` 1.13+ (for the new `errors` package)
  - `git` (to clone this project and get necessary build info)
  - `make` (to ease you life)
  - `docker` (to build container images)
- To update `CRD`s
  - __+__ `GOPATH` configured
  - __+__ Kubernetes openapi and deepcopy code generators
    - install with `make install.codegen`
  - After you have updated target structs in `pkg/apis/aranya`, run `make gen.code.all`

## Before you start

1. This porject's module name is `arhat.dev/aranya`
2. Clone this project from github

```bash
git clone https://github.com/arhat-dev/aranya

# or if you have to use go get (discouraged)
# $ go get -u arhat.dev/aranya
```

## Instructions

__NOTE:__ All binary build targets includes a `.{OS}.{ARCH}` suffix, available `OS` and `ARCH` depends on the component

### Misc

- available build related environment variables
  - `BUILD_MODE`
    - values: `default`, `exe`, `pie`

### Build `aranya`

Available `aranya` targets: `aranya.linux.{amd64,armv7,armv6,arm64}`

```bash
# build the binary directly
make aranya.linux.amd64

# or you can build in docker container
# $ make image.build.aranya.linux.amd64
```
