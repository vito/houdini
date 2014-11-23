package main_test

import (
	"os"
	"os/exec"

	linkpkg "github.com/cloudfoundry-incubator/garden-linux/old/iodaemon/link"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Iodaemon", func() {
	It("can read stdin", func() {
		spawnS, err := gexec.Start(exec.Command(
			iodaemon,
			"spawn",
			socketPath,
			"bash", "-c", "cat <&0; exit 42",
		), GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		defer spawnS.Kill()

		Eventually(spawnS).Should(gbytes.Say("ready\n"))
		Consistently(spawnS).ShouldNot(gbytes.Say("pid:"))

		linkStdout := gbytes.NewBuffer()
		link, err := linkpkg.Create(socketPath, linkStdout, os.Stderr)
		Ω(err).ShouldNot(HaveOccurred())

		link.Write([]byte("hello\ngoodbye"))
		link.Close()

		Eventually(spawnS).Should(gbytes.Say("pid:"))
		Eventually(linkStdout).Should(gbytes.Say("hello\ngoodbye"))

		Ω(link.Wait()).Should(Equal(42))
	})

	It("consistently executes a quickly-printing-and-exiting command", func() {
		for i := 0; i < 100; i++ {
			spawnS, err := gexec.Start(exec.Command(
				iodaemon,
				"spawn",
				socketPath,
				"echo", "hi",
			), GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(spawnS).Should(gbytes.Say("ready\n"))

			link := exec.Command(iodaemon, "link", socketPath)

			linkS, err := gexec.Start(link, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(spawnS).Should(gbytes.Say("pid:"))

			Eventually(linkS).Should(gbytes.Say("hi"))
			Eventually(linkS).Should(gexec.Exit(0))

			Eventually(spawnS).Should(gexec.Exit(0))

			err = os.Remove(socketPath)
			Ω(err).ShouldNot(HaveOccurred())
		}
	})
})
