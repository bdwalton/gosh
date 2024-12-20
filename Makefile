all: protos

.PHONY: protos
protos:
	@(cd protos; make)
