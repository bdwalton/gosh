PROTOC := protoc
SUBPACKAGE_FILES = $(wildcard vt/*go fragmenter/*go network/*go stm/*go)
SERVER_FILES = $(wildcard server/*go)
PROTO_OUT := protos/goshpb
GOSH_PROTO := $(PROTO_OUT)/goshpb.pb.go
VPATH := .:./protos/

all: $(GOSH_PROTO) gosh gosh-client gosh-server

gosh: gosh.go $(SUBPACKAGE_FILES) $(GOSH_PROTO)
	@echo Building gosh binary
	@go build $<

gosh-client: client/gosh-client.go $(SUBPACKAGE_FILES) $(GOSH_PROTO)
	@echo Building gosh-client binary
	@go build $<

gosh-server: $(SERVER_FILES) $(SUBPACKAGE_FILES) $(GOSH_PROTO)
	@echo Building gosh-server binary
	@(cd server; go build -o ../gosh-server)

$(GOSH_PROTO): goshpb.proto
	@echo Builing protos
	@(mkdir $(PROTO_OUT) 2>/dev/null; \
	  cd protos; \
	  $(PROTOC) --go_out=./goshpb --go_opt=paths=source_relative $(<F) )

test:
	@go test ./...

clean:
	@rm -rf gosh gosh-server gosh-client $(PROTO_OUT)
