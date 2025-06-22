# GoSH - (On the) Go Shell

Gosh is a reimplementation of mosh, but not protocol compatible with
it. It embodies the same ideas - secure UDP connection, IP roaming
within the session - but does not implement every thing mosh did. It
will do things mosh doesn't though, like ssh agent and port
forwarding.


## Building Gosh

To build Gosh, you need a go compiler that is newer than the one
specified in go.mod (1.22 as of this writing) and a protoc with the go
protoc-gen-go that is new enough to handle edition 2023 protobuf
definitions.

You should be able to pull the protoc-gen-go, if needed, from
google.golang.org via:

`go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`

The protoc compiler, if not new enough your machine already, is
available in binary form from
https://github.com/protocolbuffers/protobuf/releases

With the go, protoc and protoc-gen-go binaries in your PATH, a simple
`make` should suffice to build the three primary binaries (gosh,
gosh-server, gosh-client).
