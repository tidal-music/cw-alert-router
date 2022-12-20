FROM golang

ENV WORKDIR /app
WORKDIR ${WORKDIR}

RUN apt-get update && apt-get install -y zip

COPY ./ ${WORKDIR}

RUN GOOS=linux go test -v ./...
RUN GOOS=linux go build -o main .

RUN zip function.zip main
