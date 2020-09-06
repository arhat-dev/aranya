ARG ARCH=amd64

FROM arhatdev/builder-go:alpine as builder
FROM arhatdev/go:alpine-${ARCH}
ARG APP=template-kubernetes-controller

ENTRYPOINT [ "/template-kubernetes-controller" ]
