package houdini_test

import (
	"io"

	"github.com/cloudfoundry-incubator/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container", func() {
	var container garden.Container

	BeforeEach(func() {
		var err error
		container, err = backend.Create(garden.ContainerSpec{})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := backend.Destroy(container.Handle())
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Streaming", func() {
		Context("between containers", func() {
			var destinationContainer garden.Container

			BeforeEach(func() {
				var err error
				destinationContainer, err = backend.Create(garden.ContainerSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				err := backend.Destroy(destinationContainer.Handle())
				Expect(err).ToNot(HaveOccurred())
			})

			It("can transfer between containers", func() {
				process, err := container.Run(garden.ProcessSpec{
					Path: "sh",
					Args: []string{
						"-exc",
						`
							touch a
							touch b
							mkdir foo/
							touch foo/in-foo-a
							touch foo/in-foo-b
						`,
					},
				}, garden.ProcessIO{
					Stdout: GinkgoWriter,
					Stderr: GinkgoWriter,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(process.Wait()).To(Equal(0))

				out, err := container.StreamOut(garden.StreamOutSpec{
					Path: ".",
				})
				Expect(err).ToNot(HaveOccurred())

				err = destinationContainer.StreamIn(garden.StreamInSpec{
					Path:      ".",
					TarStream: out,
				})
				Expect(err).ToNot(HaveOccurred())

				nothing := make([]byte, 1)
				n, err := out.Read(nothing)
				Expect(n).To(Equal(0))
				Expect(err).To(Equal(io.EOF))

				checkTree, err := destinationContainer.Run(garden.ProcessSpec{
					Path: "sh",
					Args: []string{
						"-exc",
						`
							find .
							test -e a
							test -e b
							test -e foo/in-foo-a
							test -e foo/in-foo-b
						`,
					},
				}, garden.ProcessIO{
					Stdout: GinkgoWriter,
					Stderr: GinkgoWriter,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(checkTree.Wait()).To(Equal(0))
			})
		})
	})
})
