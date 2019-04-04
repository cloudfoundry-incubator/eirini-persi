FROM golang:1.12 AS build
COPY . /go/src/github.com/SUSE/eirini-extensions
ARG GO111MODULE="on"
ENV GO111MODULE $GO111MODULE
RUN cd /go/src/github.com/SUSE/eirini-extensions && \
    make build && \
    cp -p binaries/eirni-ext /usr/local/bin/eirini-ext

FROM opensuse/leap:15.0
RUN zypper -n in system-user-nobody
USER nobody
COPY --from=build /usr/local/bin/eirini-ext /usr/local/bin/eirini-ext
ENTRYPOINT ["eirini-ext"]
