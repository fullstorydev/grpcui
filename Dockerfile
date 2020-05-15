FROM golang:1.13-alpine as builder
MAINTAINER FullStory Engineering

# currently, a module build requires gcc (so Go tool can build
# module-aware versions of std library; it ships only w/ the
# non-module versions)
RUN apk update && apk add --no-cache ca-certificates git gcc g++ libc-dev
# create non-privileged group and user
RUN addgroup -S grpcui && adduser -S grpcui -G grpcui

WORKDIR /tmp/fullstorydev/grpcui
# copy just the files/sources we need to build grpcui
COPY VERSION *.go go.* /tmp/fullstorydev/grpcui/
COPY cmd /tmp/fullstorydev/grpcui/cmd
COPY internal /tmp/fullstorydev/grpcui/internal
COPY standalone /tmp/fullstorydev/grpcui/standalone
# and build a completely static binary (so we can use
# scratch as basis for the final image)
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GO111MODULE=on
RUN go build -o /grpcui \
    -ldflags "-w -extldflags \"-static\" -X \"main.version=$(cat VERSION)\"" \
    ./cmd/grpcui

# New FROM so we have a nice'n'tiny image
FROM scratch
WORKDIR /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /grpcui /bin/grpcui
USER grpcui
EXPOSE 8080

ENTRYPOINT ["/bin/grpcui", "-bind=0.0.0.0", "-port=8080"]
