# Installation

All `arhat` binaries except `arhat-libpod` are self contained and able to run with proper configuration (see [docs/arhat-config.md](./arhat-config.md))

## Additional steps to install `arhat-libpod`

- Install system dependencies required by libpod
  - debian family
    - libraries
      - `libgpgme11`
      - `libseccomp2`
      - `libdevmapper1.02.1`
      - `libglib2.0`
      - `ca-certificates`
    - binaries
      - `conmon`
        - You can extract it from the docker image `docker.io/arhatdev/conmon:{amd64,armv6,armv7,arm64}`
      - `runc`
        - You can extract it from the docker image `docker.io/arhatdev/runc:{amd64,armv6,armv7,arm64}`
  - alpine
    - libraries
      - `gpgme`
      - `libseccomp`
- Download and deploy `arhat-libpod`
  - You can extract one from the docker image `docker.io/arhatdev/arhat-libpod:{amd64,armv6,armv7,arm64}`
  - Deploy to your host and run with root privilege (rootless is not supported for now)
- Configure `arhat-libpod` runtime with libpod configurations (see https://podman.io/getting-started/installation#configuration-files)

__Note:__ to extract binary from image, you need a container image management tool
example for `docker`:

```bash
export ID="$(docker create ${IMAGE_NAME})"
docker cp ${ID}:/arhat-libpod ${BINARY}
docker rm -f ${ID}
```
