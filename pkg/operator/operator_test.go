package operator_test

import (
	"github.com/SUSE/eirini-extensions/pkg/bosh/manifest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("operator", func() {
	Describe("GetOperatorDockerImage", func() {
		It("returns the location of the docker image", func() {
			manifest.DockerTag = "1.2.3"
			manifest.DockerOrganization = "foo"
			manifest.DockerRepository = "bar"

			Expect(manifest.GetOperatorDockerImage()).To(Equal("foo/bar:1.2.3"))
		})
	})
})
