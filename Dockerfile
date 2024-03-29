FROM golang:1.16 AS build
ARG TARGETARCH
ARG TARGETOS
ARG arg_application=INVALID
ENV env_application=$arg_application
WORKDIR /git
COPY *.go /git/.
COPY cmd/ /git/cmd/.
COPY app/ /git/app/.
COPY go.mod /git/.
COPY go.sum /git/.
RUN mkdir -p bin/
RUN go build -o bin/ ./cmd/$env_application

FROM alpine:3 AS image
ARG TARGETARCH
ARG TARGETOS
ARG arg_application=INVALID
WORKDIR /git
ENV env_application=$arg_application
RUN echo build docker image for $env_application running on $TARGETARCH
RUN apk update
RUN apk add libc6-compat

RUN mkdir -p /usr/local/share/phoenix/config
COPY --from=build /git/bin/$env_application /usr/local/share/phoenix/
WORKDIR /usr/local/share/phoenix
CMD /usr/local/share/phoenix/$env_application

