all: protos gosh gosh-client gosh-server

SUBPACKAGE_FILES = $(wildcard vt/*go fragmenter/*go network/*go stm/*go)

gosh: gosh.go $(SUBPACKAGE_FILES)
	@echo Building $<
	@go build $<

gosh-client: client/gosh-client.go $(SUBPACKAGE_FILES)
	@echo Building $<
	@go build $<

gosh-server: server/gosh-server.go $(SUBPACKAGE_FILES)
	@echo Building $<
	@go build $<

.PHONY: protos clean
protos:
	@(cd protos; make)

clean:
	@rm gosh gosh-server gosh-client
