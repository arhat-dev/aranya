ARG ARCH=amd64

FROM arhatdev/builder-go:alpine as builder
FROM arhatdev/go:alpine-${ARCH}

ADD cicd/config/sample-host-key.pem /etc/ssh/ssh_host_ed25519_key
ADD cicd/config/sample-host-key.pem.pub /etc/ssh/ssh_host_ed25519_key.pub

ENTRYPOINT [ "/aranya" ]
