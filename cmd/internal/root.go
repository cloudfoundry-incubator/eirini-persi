package cmd

import (
	golog "log"
	"os"

	"github.com/SUSE/eirini-persi/version"
	eirinix "github.com/SUSE/eirinix"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	persistence "github.com/SUSE/eirini-persi/extensions/persistence"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // from https://github.com/kubernetes/client-go/issues/345
)

var (
	log *zap.SugaredLogger
)

var rootCmd = &cobra.Command{
	Use:   "eirini-ext",
	Short: "eirini-ext-operator manages Eirini apps on Kubernetes",
	Run: func(cmd *cobra.Command, args []string) {
		defer log.Sync()
		kubeConfig := viper.GetString("kubeconfig")

		namespace := viper.GetString("namespace")

		log.Infof("Starting %s with namespace %s", version.Version, namespace)

		webhookHost := viper.GetString("operator-webhook-host")
		webhookPort := viper.GetInt32("operator-webhook-port")
		serviceName := viper.GetString("operator-service-name")
		webhookNamespace := viper.GetString("operator-webhook-namespace")

		if webhookHost == "" {
			log.Fatal("required flag 'operator-webhook-host' not set (env variable: OPERATOR_WEBHOOK_HOST)")
		}
		x := eirinix.NewManager(
			eirinix.ManagerOptions{
				Namespace:        namespace,
				Host:             webhookHost,
				Port:             webhookPort,
				KubeConfig:       kubeConfig,
				ServiceName:      serviceName,
				WebhookNamespace: webhookNamespace,
			})

		x.AddExtension(persistence.New())
		log.Fatal(x.Start())
	},
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
	pf.StringP("operator-service-name", "s", "eirini-persi", "Service name where the webhook runs on (Optional, only needed inside kube)")
	pf.StringP("operator-webhook-namespace", "t", "cf", "The namespace the services lives in (Optional, only needed inside kube)")

	viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	viper.BindPFlag("namespace", pf.Lookup("namespace"))
	viper.BindPFlag("operator-webhook-host", pf.Lookup("operator-webhook-host"))
	viper.BindPFlag("operator-webhook-port", pf.Lookup("operator-webhook-port"))
	viper.BindPFlag("operator-service-name", pf.Lookup("operator-service-name"))
	viper.BindPFlag("operator-webhook-namespace", pf.Lookup("operator-webhook-namespace"))

	viper.BindEnv("kubeconfig")
	viper.BindEnv("namespace", "NAMESPACE")
	viper.BindEnv("operator-webhook-host", "OPERATOR_WEBHOOK_HOST")
	viper.BindEnv("operator-webhook-port", "OPERATOR_WEBHOOK_PORT")
	viper.BindEnv("operator-service-name", "OPERATOR_SERVICE_NAME")
	viper.BindEnv("operator-webhook-namespace", "OPERATOR_WEBHOOK_NAMESPACE")
}

// initConfig is executed before running commands
func initConfig() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		golog.Fatalf("cannot initialize ZAP logger: %v", err)
	}
	log = logger.Sugar()
}
