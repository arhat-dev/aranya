ARG ARCH=armv5

FROM ghcr.io/arhat-dev/builder-go:alpine as builder
FROM ghcr.io/arhat-dev/go:debian-${ARCH}

ENTRYPOINT [ "/aranya" ]
