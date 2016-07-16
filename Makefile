.PHONY: deps clean

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
	rm -rf Godeps_linux

Godeps_windows:
	mkdir Godeps_windows

Godeps_darwin:
	mkdir Godeps_darwin

Godeps_linux:
	mkdir Godeps_linux

deps: Godeps_windows Godeps_darwin Godeps_linux
	GOOS=windows godep save ./... && mv Godeps/* Godeps_windows && rmdir Godeps
	GOOS=darwin godep save ./... && mv Godeps/* Godeps_darwin && rmdir Godeps
	GOOS=linux godep save ./... && mv Godeps/* Godeps_linux && rmdir Godeps
	mkdir -p deps
	rm -rf deps/src
	mv vendor deps/src
