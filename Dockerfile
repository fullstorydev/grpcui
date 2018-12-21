FROM golang:1.10
ADD . /go/src/github.com/fullstorydev/grpcui
WORKDIR /go/src/github.com/fullstorydev/grpcui
RUN make deps install
