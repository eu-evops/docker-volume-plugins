FROM alpine:3.10.3 as base

FROM base AS build
RUN apk add go git musl-dev ca-certificates

ADD . /code

RUN go get github.com/boltdb/bolt github.com/docker/go-plugins-helpers/volume && \
    cd /code/azure-storage-volume-plugin && \
    go build -o azure-storage-volume-plugin main.go  && \
    ls -lart .


FROM base as release

COPY --from=build /code/azure-storage-volume-plugin/azure-storage-volume-plugin /

RUN apk add ca-certificates && update-ca-certificates && ls -la /*plugin
