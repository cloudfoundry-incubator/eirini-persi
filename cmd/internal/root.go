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
		register := viper.GetBool("register-only")
		start := viper.GetBool("start-only")

		if webhookHost == "" {
			log.Fatal("required flag 'operator-webhook-host' not set (env variable: OPERATOR_WEBHOOK_HOST)")
		}

		RegisterWebhooks := true
		if start {
			log.Info("start-only supplied, the extension will start without registering")
			RegisterWebhooks = false
		}

		x := eirinix.NewManager(
			eirinix.ManagerOptions{
				Namespace:        namespace,
				Host:             webhookHost,
				Port:             webhookPort,
				KubeConfig:       kubeConfig,
				ServiceName:      serviceName,
				WebhookNamespace: webhookNamespace,
				RegisterWebHook:  &RegisterWebhooks,
			})

		x.AddExtension(persistence.New())

		if register {
			log.Info("Registering the extension")
			err := x.RegisterExtensions()
			if err != nil {
				log.Fatal(err.Error())
			}
			return
		}
		log.Info("Starting the extension")

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
	pf.StringP("operator-service-name", "s", "", "Service name where the webhook runs on (Optional, only needed inside kube)")
	pf.StringP("operator-webhook-namespace", "t", "", "The namespace the services lives in (Optional, only needed inside kube)")
	pf.BoolP("register-only", "", false, "Register the extension, do not start it (default: register and start)")
	pf.BoolP("start-only", "", false, "Starts the extension, do not register it (default: register and start)")

	viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	viper.BindPFlag("namespace", pf.Lookup("namespace"))
	viper.BindPFlag("operator-webhook-host", pf.Lookup("operator-webhook-host"))
	viper.BindPFlag("operator-webhook-port", pf.Lookup("operator-webhook-port"))
	viper.BindPFlag("operator-service-name", pf.Lookup("operator-service-name"))
	viper.BindPFlag("operator-webhook-namespace", pf.Lookup("operator-webhook-namespace"))
	viper.BindPFlag("register-only", pf.Lookup("register-only"))
	viper.BindPFlag("start-only", pf.Lookup("start-only"))

	viper.BindEnv("kubeconfig")
	viper.BindEnv("namespace", "NAMESPACE")
	viper.BindEnv("operator-webhook-host", "OPERATOR_WEBHOOK_HOST")
	viper.BindEnv("operator-webhook-port", "OPERATOR_WEBHOOK_PORT")
	viper.BindEnv("operator-service-name", "OPERATOR_SERVICE_NAME")
	viper.BindEnv("operator-webhook-namespace", "OPERATOR_WEBHOOK_NAMESPACE")
	viper.BindEnv("register-only", "EIRINI_EXTENSION_REGISTER_ONLY")
	viper.BindEnv("start-only", "EIRINI_EXTENSION_START_ONLY")
}

// initConfig is executed before running commands
func initConfig() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		golog.Fatalf("cannot initialize ZAP logger: %v", err)
	}
	log = logger.Sugar()
}
