package houdini

import (
	"code.cloudfoundry.org/garden/server"
)

// re-export vendored Garden server to work around vendor/ issue
var NewServer = server.New
