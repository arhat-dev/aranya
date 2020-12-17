FROM ghcr.io/arhat-dev/builder-go:alpine as builder
FROM ghcr.io/arhat-dev/go:scratch

COPY e2e/testdata/ssh-host-key.pem /etc/ssh/ssh_host_ed25519_key
COPY e2e/testdata/ssh-host-key.pem.pub /etc/ssh/ssh_host_ed25519_key.pub

VOLUME [ "/output" ]

ENTRYPOINT [ "/aranya", "-coverprofile=/output/coverage.image.txt", "--" ]
