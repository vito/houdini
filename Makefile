skeleton: skeleton/workdir skeleton/bin/iodaemon

skeleton/bin/iodaemon: skeleton/bin
	go build -o skeleton/bin/iodaemon ./iodaemon/

skeleton/bin:
	mkdir -p skeleton/bin

skeleton/workdir:
	mkdir -p skeleton/workdir
