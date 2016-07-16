package houdini

import (
	"github.com/cloudfoundry-incubator/garden/server"
)

// re-export vendored Garden server to work around vendor/ issue
var NewServer = server.New
