skeleton: skeleton/workdir skeleton/tmpdir skeleton/bin/iodaemon

skeleton/bin/iodaemon: skeleton/bin iodaemon/**/*
	go build -o skeleton/bin/iodaemon ./iodaemon/

skeleton/bin:
	mkdir -p skeleton/bin

skeleton/workdir:
	mkdir -p skeleton/workdir

skeleton/tmpdir:
	mkdir -p skeleton/tmpdir
