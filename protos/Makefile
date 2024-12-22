PROTOC := protoc

PB_GOS := $(patsubst %.proto,%.pb.go,$(wildcard *proto))

all: $(PB_GOS)

$(PB_GOS): %.pb.go : %.proto
	@[ -d "$*" ] || mkdir $*
	@$(PROTOC) --go_out=./$* --go_opt=paths=source_relative $<

.PHONY: clean
clean:
	@rm -f */*pb.go
