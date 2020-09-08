package cmd

import (
	"os"

	"code.cloudfoundry.org/eirini-persi/version"
	eirinix "code.cloudfoundry.org/eirinix"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	persistence "code.cloudfoundry.org/eirini-persi/extensions/persistence"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // from https://github.com/kubernetes/client-go/issues/345
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register the eirini extension",
	PreRun: func(cmd *cobra.Command, args []string) {

		viper.BindPFlag("kubeconfig", cmd.Flags().Lookup("kubeconfig"))
		viper.BindPFlag("namespace", cmd.Flags().Lookup("namespace"))
		viper.BindPFlag("operator-webhook-host", cmd.Flags().Lookup("operator-webhook-host"))
		viper.BindPFlag("operator-webhook-port", cmd.Flags().Lookup("operator-webhook-port"))
		viper.BindPFlag("operator-service-name", cmd.Flags().Lookup("operator-service-name"))
		viper.BindPFlag("operator-webhook-namespace", cmd.Flags().Lookup("operator-webhook-namespace"))

		viper.BindEnv("kubeconfig")
		viper.BindEnv("namespace", "NAMESPACE")
		viper.BindEnv("operator-webhook-host", "OPERATOR_WEBHOOK_HOST")
		viper.BindEnv("operator-webhook-port", "OPERATOR_WEBHOOK_PORT")
		viper.BindEnv("operator-service-name", "OPERATOR_SERVICE_NAME")
		viper.BindEnv("operator-webhook-namespace", "OPERATOR_WEBHOOK_NAMESPACE")
	},
	Run: func(cmd *cobra.Command, args []string) {
		defer log.Sync()
		kubeConfig := viper.GetString("kubeconfig")

		namespace := viper.GetString("namespace")

		log.Infof("Registering %s with namespace %s", version.Version, namespace)

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

		if err := x.RegisterExtensions(); err != nil {
			log.Fatal(err.Error())
		}
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(registerCmd)
}
