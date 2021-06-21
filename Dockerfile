FROM alpine:3
ARG arg_application=INVALID
ARG arg_arch=amd64
ENV env_application=$arg_application
ENV env_arch=$arg_arch
RUN echo build docker image for $env_application
RUN apk update
RUN apk add libc6-compat

RUN mkdir -p /usr/local/share/phoenix/config
COPY bin/$env_arch/$env_application /usr/local/share/phoenix/
WORKDIR /usr/local/share/phoenix
CMD /usr/local/share/phoenix/$env_application

