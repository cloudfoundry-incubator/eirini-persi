package cmd

import (
	golog "log"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // from https://github.com/kubernetes/client-go/issues/345
)

var (
	log *zap.SugaredLogger
)

var rootCmd = &cobra.Command{
	Use:   "eirini-ext",
	Short: "eirini-ext-operator manages Eirini apps on Kubernetes",
}

// Execute the root command, runs the server
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		golog.Fatal(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	pf := rootCmd.PersistentFlags()

	pf.StringP("kubeconfig", "c", "", "Path to a kubeconfig, not required in-cluster")
	pf.StringP("namespace", "n", "eirini", "Namespace to watch for Eirini apps")
	pf.StringP("operator-webhook-host", "w", "", "Hostname/IP under which the webhook server can be reached from the cluster")
	pf.StringP("operator-webhook-port", "p", "2999", "Port the webhook server listens on")
	pf.StringP("operator-service-name", "s", "", "Service name where the webhook runs on (Optional, only needed inside kube)")
	pf.StringP("operator-webhook-namespace", "t", "", "The namespace the services lives in (Optional, only needed inside kube)")
}

// initConfig is executed before running commands
func initConfig() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		golog.Fatalf("cannot initialize ZAP logger: %v", err)
	}
	log = logger.Sugar()
}
