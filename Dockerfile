FROM golang as base

ENV WORKDIR /app
WORKDIR ${WORKDIR}

COPY ./go.* ${WORKDIR}/
RUN go mod download

COPY ./ ${WORKDIR}/

FROM base as build

RUN apt-get update && apt-get install -y zip

RUN go build -o main .

RUN zip function.zip main
