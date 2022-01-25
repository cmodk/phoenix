FROM golang:1.16 AS build
ARG TARGETARCH
ARG TARGETOS
WORKDIR /git
COPY *.go /git/.
COPY cmd/ /git/cmd/.
COPY app/ /git/app/.
COPY go.mod /git/.
COPY go.sum /git/.
RUN go build -o bin/ ./cmd/*

FROM alpine:3 AS image
WORKDIR /git
RUN apk update
RUN apk add libc6-compat

RUN mkdir -p /usr/local/share/phoenix/config
COPY --from=build /git/bin/* /usr/local/share/phoenix/
WORKDIR /usr/local/share/phoenix
CMD /usr/local/share/phoenix/phoenix-helper

