# MUST use debian image for race test and GNU tar for `kubectl cp`
FROM ghcr.io/arhat-dev/builder-go:debian as builder
FROM ghcr.io/arhat-dev/go:debian

COPY e2e/testdata/ssh-host-key.pem /etc/ssh/ssh_host_ed25519_key
COPY e2e/testdata/ssh-host-key.pem.pub /etc/ssh/ssh_host_ed25519_key.pub

COPY e2e/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT [ \
    "/entrypoint.sh", "/aranya", \
    "-test.blockprofile=/profile/blockprofile.out", \
    "-test.cpuprofile=/profile/cpuprofile.out", \
    "-test.memprofile=/profile/memprofile.out", \
    "-test.mutexprofile=/profile/mutexprofile.out", \
    "-test.coverprofile=/profile/coverage.txt", \
    "-test.outputdir=/profile", \
    "--" \
]
