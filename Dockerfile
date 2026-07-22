FROM golang:1.24 AS base

ENV WORKDIR=/app
WORKDIR ${WORKDIR}

COPY ./go.* ${WORKDIR}/
RUN go mod download

COPY ./ ${WORKDIR}/

FROM base AS build

RUN apt-get update && apt-get install -y zip

# The provided.al2023 lambda runtime requires the binary to be named "bootstrap".
# lambda.norpc drops the legacy RPC support only needed by the old go1.x runtime.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap .

RUN zip function.zip bootstrap
