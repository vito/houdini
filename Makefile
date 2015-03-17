darwin.tar.gz: dist/houdini dist/skeleton
	tar vczf darwin.tar.gz -C dist houdini skeleton

dist/houdini: cmd/houdini/**/*
	go build -o dist/houdini ./cmd/houdini

dist/skeleton: dist/skeleton/workdir dist/skeleton/bin/iodaemon

dist/skeleton/bin/iodaemon: dist/skeleton/bin iodaemon/**/*
	go build -o dist/skeleton/bin/iodaemon ./iodaemon/

dist/skeleton/bin:
	mkdir -p dist/skeleton/bin

dist/skeleton/workdir:
	mkdir -p dist/skeleton/workdir
