ARG ARCH=mips64le

FROM ghcr.io/arhat-dev/builder-go:alpine as builder
FROM ghcr.io/arhat-dev/go:debian-${ARCH}

ENTRYPOINT [ "/aranya" ]
