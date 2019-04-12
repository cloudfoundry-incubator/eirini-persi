package e2e_test

import (
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("CLI", func() {
	act := func(arg ...string) (session *gexec.Session, err error) {
		cmd := exec.Command(cliPath, arg...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		return
	}

	Describe("help", func() {
		It("should show the help for server", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Usage:`))
		})

		It("should show all available options for server", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Flags:
  -h, --help                             help for eirini-extensions
  -c, --kubeconfig string                Path to a kubeconfig, not required in-cluster
  -n, --namespace string                 Namespace to watch for Eirini apps \(default "eirini"\)
  -w, --operator-webhook-host string     Hostname/IP under which the webhook server can be reached from the cluster
  -p, --operator-webhook-port string     Port the webhook server listens on \(default "2999"\)`))
		})

		It("shows all available commands", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Available Commands:
  help                   Help about any command
  version                Print the version number

`))
		})
	})

	Describe("default", func() {
		It("should start the server", func() {
			session, err := act()
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Err).Should(Say(`Starting eirini-extensions \d+\.\d+\.\d+ with namespace`))
		})

		Context("when specifying namespace", func() {
			Context("via environment variables", func() {
				BeforeEach(func() {
					os.Setenv("NAMESPACE", "env-test")
				})

				AfterEach(func() {
					os.Setenv("NAMESPACE", "")
				})

				It("should start for namespace", func() {
					session, err := act()
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Starting eirini-extensions \d+\.\d+\.\d+ with namespace env-test`))
				})
			})

			Context("via using switches", func() {
				It("should start for namespace", func() {
					session, err := act("--namespace", "switch-test")
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Starting eirini-extensions \d+\.\d+\.\d+ with namespace switch-test`))
				})
			})
		})
	})

	Describe("version", func() {
		It("should show a semantic version number", func() {
			session, err := act("version")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Eirini Extensions Operator Version: \d+.\d+.\d+`))
		})
	})
})
