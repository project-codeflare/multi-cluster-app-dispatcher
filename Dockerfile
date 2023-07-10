FROM registry.access.redhat.com/ubi8/go-toolset:1.18.10-1 AS BUILDER
ARG GO_BUILD_ARGS
WORKDIR /workdir
USER root

COPY Makefile Makefile
COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd cmd
COPY pkg pkg
COPY hack hack

RUN cd /workdir && curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/$(go env GOARCH)/kubectl && chmod +x kubectl
ENV GO_BUILD_ARGS=$GO_BUILD_ARGS
RUN echo "Go build args: $GO_BUILD_ARGS" && \
    make mcad-controller

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

COPY --from=BUILDER /workdir/_output/bin/mcad-controller /usr/local/bin
COPY --from=BUILDER /workdir/kubectl /usr/local/bin

RUN true \
    && microdnf update \
    && microdnf clean all \
    && true

WORKDIR /usr/local/bin

RUN chown -R 1000:1000 /usr/local/bin

USER 1000
