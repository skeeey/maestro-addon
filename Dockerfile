FROM registry.ci.openshift.org/stolostron/builder:go1.23-linux AS builder

ARG OS=linux
ARG ARCH=amd64

WORKDIR /go/src/github.com/stolostron/maestro-addon
COPY . .
ENV GO_PACKAGE github.com/stolostron/maestro-addon

RUN GOOS=${OS} \
    GOARCH=${ARCH} \
    make build --warn-undefined-variables

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
ENV USER_UID=10001

COPY --from=builder /go/src/github.com/stolostron/maestro-addon/maestroaddon /usr/local/bin/

USER ${USER_UID}
