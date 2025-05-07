# GoSH - (On the) Go Shell

Gosh is a reimplementation of mosh, but not protocol compatible with it. It
embodies the same ideas - secure UDP connection with IP roaming within the
session but does not implement every thing mosh did. For now, it focuses on the
roaming abilities without the local echo feature.

# Building

To build Gosh, you need a go compiler that is newer than the one specified in
go.mod (1.22 as of this writing) and a protoc with the go protoc-gen-go that are
new enough to handle edition 2023 protobuf definitions.

You should be able to pull the protoc-gen-go from google.golang.org via 

`go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`

The protoc compiler, if not new enough your machine already is available in
binary form from https://github.com/protocolbuffers/protobuf

From there, a simple `make` should suffice to build the three primary binaries.
