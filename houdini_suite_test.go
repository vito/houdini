package houdini_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vito/houdini"

	"testing"
)

var depotDir string
var backend *houdini.Backend

var _ = BeforeEach(func() {
	var err error
	depotDir, err = ioutil.TempDir("", "depot")
	Expect(err).ToNot(HaveOccurred())

	backend = houdini.NewBackend(depotDir)

	err = backend.Start()
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterEach(func() {
	backend.Stop()

	err := os.RemoveAll(depotDir)
	Expect(err).ToNot(HaveOccurred())
})

func TestHoudini(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Houdini Suite")
}
