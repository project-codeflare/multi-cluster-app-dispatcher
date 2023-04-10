FROM  registry.access.redhat.com/ubi8/go-toolset:1.18.10-1 AS BUILDER
USER root
WORKDIR /workdir

COPY Makefile Makefile
COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd cmd
COPY pkg pkg
COPY hack hack

RUN make mcad-controller

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

COPY --from=BUILDER /workdir/_output/bin/mcad-controller /usr/local/bin

RUN true \
    && microdnf update \
    && microdnf --nodocs install \
        curl shadow-utils \
    && microdnf clean all \
    && true

RUN cd /usr/local/bin && curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x kubectl

WORKDIR /usr/local/bin

RUN groupadd --system --gid=9999 mcad && \
    useradd --system --create-home --uid=9999 --gid=mcad mcad

RUN chown -R mcad:mcad /usr/local/bin

USER mcad
