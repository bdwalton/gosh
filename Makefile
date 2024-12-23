all: protos gosh gosh-client gosh-server

gosh: gosh.go
	@echo Building $<
	@go build $<

gosh-client: client/gosh-client.go
	@echo Building $<
	@go build $<

gosh-server: server/gosh-server.go
	@echo Building $<
	@go build $<

.PHONY: protos clean
protos:
	@(cd protos; make)

clean:
	@rm gosh gosh-server gosh-client
