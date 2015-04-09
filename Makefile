darwin.tar.gz: dist/houdini dist/skeleton
	tar vczf darwin.tar.gz -C dist houdini skeleton

dist/houdini: cmd/houdini/**/*
	go build -o dist/houdini ./cmd/houdini

dist/skeleton/bin:
	mkdir -p dist/skeleton/bin

dist/skeleton/workdir:
	mkdir -p dist/skeleton/workdir

clean:
	rm -rf dist
	rm -rf Godeps_windows
	rm -rf Godeps_darwin

deps:
	GOOS=windows godep save ./... && mv Godeps Godeps_windows
	GOOS=darwin godep save ./... && mv Godeps Godeps_darwin
