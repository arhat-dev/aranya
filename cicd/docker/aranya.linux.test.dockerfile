FROM ghcr.io/arhat-dev/builder-go:alpine as builder
FROM ghcr.io/arhat-dev/go:alpine

COPY e2e/testdata/ssh-host-key.pem /etc/ssh/ssh_host_ed25519_key
COPY e2e/testdata/ssh-host-key.pem.pub /etc/ssh/ssh_host_ed25519_key.pub

RUN mkdir -p /output

ENTRYPOINT [ \
    "/aranya", \
    "-test.blockprofile=/output/blockprofile.txt", \
    "-test.cpuprofile=/output/cpuprofile.txt", \
    "-test.memprofile=/output/memprofile.txt", \
    "-test.mutexprofile=/output/mutexprofile.txt", \
    "-test.coverprofile=/output/coverage.txt", \
    "-test.outputdir=/output", \
    "--" \
]
