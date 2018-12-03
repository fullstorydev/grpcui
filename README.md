# gRPC UI
[![Build Status](https://travis-ci.org/fullstorydev/grpcui.svg?branch=master)](https://travis-ci.org/fullstorydev/grpcui/branches)
[![Go Report Card](https://goreportcard.com/badge/github.com/fullstorydev/grpcui)](https://goreportcard.com/report/github.com/fullstorydev/grpcui)

`grpcui` is a command-line tool that lets you interact with gRPC servers via a browser.
It's sort of like [Postman](https://www.getpostman.com/), but for gRPC APIs instead of
REST.

In some ways, this is like an extension to [grpcurl](https://github.com/fullstorydev/grpcurl).
Whereas `grpcurl` is a command-line interface, `grpcui` provides a web/browser-based
GUI. This lets you interactively construct requests to send to a gRPC server.

With this tool you can also browse the schema for gRPC services, which is presented as a
list of available endpoints. This is enabled either by querying a server that supports
[server reflection](https://github.com/grpc/grpc/blob/master/src/proto/grpc/reflection/v1alpha/reflection.proto),
by reading proto source files, or by loading in compiled "protoset" files (files that contain
encoded file [descriptor protos](https://github.com/google/protobuf/blob/master/src/google/protobuf/descriptor.proto)).
In fact, the way the tool transforms JSON request data into a binary encoded protobuf
is using that very same schema. So, if the server you interact with does not support
reflection, you will either need the proto source files that define the service or need
protoset files that `grpcui` can use.

This repo also provides two library packages
1. `github.com/fullstorydev/grpcui`: This package contains the building blocks for embedding a
   gRPC web form into any Go HTTP server. It has functions for accessing the HTML form, the
   JavaScript code that powers it, as well as a sample CSS file, for styling the form.
2. `github.com/fullstorydev/grpcui/standalone`: This package goes a step further and supplies
   a single, simple HTTP handler that provides the entire gRPC web UI. You can just wire this
   handler into your HTTP server to embed a gRPC web page that looks exactly like the one you
   see when you use the `grpcui` command-line program. This single handler uses the above
   package but also supplies the enclosing HTML page, some other script dependencies (jQuery
   and jQuery-UI), and additional CSS and image resources.

## Features
`grpcui` supports all kinds of RPC methods, including streaming methods. However, it requires
you to construct the entire stream of request messages all at once and then renders the entire
resulting stream of response messages all at once (so you can't interact with bidirectional
streams the way that `grpcurl` can).

`grpcui` supports both plain-text and TLS servers and has numerous options for TLS
configuration. It also supports mutual TLS, where the client is required to present a
client certificate.

As mentioned above, `grpcui` works seamlessly if the server supports the reflection
service. If not, you can supply the `.proto` source files or you can supply protoset
files (containing compiled descriptors, produced by `protoc`) to `grpcui`.

The web UI allows you to set request metadata in addition to defining the request message data.
When defining request message data, it uses a dynamic HTML form that supports data entry for
all possible kinds of protobuf messages, including rich support for well-known types (such as
`google.protobuf.Timestamp`), one ofs, and maps.

In addition to entering the data via HTML form, you can also enter the data in JSON format,
by typing or pasting the entire JSON request body into a text form.

Upon issuing an RPC, the web UI shows all gRPC response metadata, including both headers and
trailers sent by the server. And, of course, it shows a human-comprehensible response body, in
the form of an HTML table.

## Installation

### From Source
You can use the `go` tool to install `grpcui`:
```shell
go get github.com/fullstorydev/grpcui
go install github.com/fullstorydev/grpcui/cmd/grpcui
```

This installs the command into the `bin` sub-folder of wherever your `$GOPATH`
environment variable points. If this directory is already in your `$PATH`, then
you should be good to go.

If you have already pulled down this repo to a location that is not in your
`$GOPATH` and want to build from the sources, you can `cd` into the repo and then
run `make install`.

If you encounter compile errors, you could have out-dated versions of `grpcui`'s
dependencies. You can update the dependencies by running `make updatedeps`.

## Usage
The usage doc for the tool explains the numerous options:
```shell
grpcui -help
```

Most of the flags control how the program connects to the gRPC server that to which
requests will be sent. However, there is one flag that controls `grpcui` itself: the
`-port` flag controls what port the HTTP server should use to expose the web UI. If
no port is specified, an ephemeral port will be used (so likely a different port each
time it is run, allocated by the operating system).

### Web Form
*TODO(jhump)*: Describe how to define requests; include screenshots

### Raw JSON Requests
*TODO(jhump)*: Describe how to examine and even edit the raw JSON for requests; include screenshots

### RPC Results
*TODO(jhump)*: Describe how results are presented; include screenshots

## Descriptor Sources
The `grpcui` tool can operate on a variety of sources for descriptors. The descriptors
are required, in order for `grpcui` to understand the RPC schema, translate inputs
into the protobuf binary format as well as translate responses from the binary format
into text. The sections below document the supported sources and what command-line flags
are needed to use them.

### Server Reflection

Without any additional command-line flags, `grpcui` will try to use [server reflection](https://github.com/grpc/grpc/blob/master/src/proto/grpc/reflection/v1alpha/reflection.proto).

Examples for how to set up server reflection can be found [here](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md#known-implementations).

### Proto Source Files
To use `grpcui` on servers that do not support reflection, you can use `.proto` source
files.

In addition to using `-proto` flags to point `grpcui` at the relevant proto source file(s),
you may also need to supply `-import-path` flags to tell `grpcui` the folders from which
dependencies can be imported.

Just like when compiling with `protoc`, you do *not* need to provide an import path for the
location of the standard protos included with `protoc` (which contain various "well-known
types" with a package definition of `google.protobuf`). These files are "known" by `grpcui`
as a snapshot of their descriptors is built into the `grpcui` binary.

### Protoset Files
You can also use compiled protoset files with `grpcui`. Protoset files contain binary
encoded `google.protobuf.FileDescriptorSet` protos. To create a protoset file, invoke
`protoc` with the `*.proto` files that define the service:

```shell
protoc --proto_path=. \
    --descriptor_set_out=myservice.protoset \
    --include_imports \
    my/custom/server/service.proto
```

The `--descriptor_set_out` argument is what tells `protoc` to produce a protoset,
and the `--include_imports` argument is necessary for the protoset to contain
everything that `grpcui` needs to process and understand the schema.
