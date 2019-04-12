package cmd

import (
	golog "log"
	"os"
	"time"

	kubeConfig "code.cloudfoundry.org/cf-operator/pkg/kube/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"github.com/SUSE/eirini-extensions/pkg/operator"
	"github.com/SUSE/eirini-extensions/pkg/util/ctxlog"
	"github.com/SUSE/eirini-extensions/version"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // from https://github.com/kubernetes/client-go/issues/345
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

var (
	log *zap.SugaredLogger
)

var rootCmd = &cobra.Command{
	Use:   "eirini-ext",
	Short: "eirini-ext-operator manages Eirini apps on Kubernetes",
	Run: func(cmd *cobra.Command, args []string) {
		defer log.Sync()

		restConfig, err := kubeConfig.NewGetter(log).Get(viper.GetString("kubeconfig"))
		if err != nil {
			log.Fatal(err)
		}
		if err := kubeConfig.NewChecker(log).Check(restConfig); err != nil {
			log.Fatal(err)
		}

		namespace := viper.GetString("namespace")

		log.Infof("Starting %s with namespace %s", version.Version, namespace)

		webhookHost := viper.GetString("operator-webhook-host")
		webhookPort := viper.GetInt32("operator-webhook-port")

		if webhookHost == "" {
			log.Fatal("required flag 'operator-webhook-host' not set (env variable: OPERATOR_WEBHOOK_HOST)")
		}

		config := &config.Config{
			CtxTimeOut:        10 * time.Second,
			Namespace:         namespace,
			WebhookServerHost: webhookHost,
			WebhookServerPort: webhookPort,
			Fs:                afero.NewOsFs(),
		}
		ctx := ctxlog.NewManagerContext(log)

		mgr, err := operator.NewManager(ctx, config, restConfig, manager.Options{Namespace: namespace})
		if err != nil {
			log.Fatal(err)
		}

		log.Fatal(mgr.Start(signals.SetupSignalHandler()))
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
	viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	viper.BindPFlag("namespace", pf.Lookup("namespace"))
	viper.BindPFlag("operator-webhook-host", pf.Lookup("operator-webhook-host"))
	viper.BindPFlag("operator-webhook-port", pf.Lookup("operator-webhook-port"))
	viper.BindEnv("kubeconfig")
	viper.BindEnv("namespace", "NAMESPACE")
	viper.BindEnv("operator-webhook-host", "OPERATOR_WEBHOOK_HOST")
	viper.BindEnv("operator-webhook-port", "OPERATOR_WEBHOOK_PORT")
}

// initConfig is executed before running commands
func initConfig() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		golog.Fatalf("cannot initialize ZAP logger: %v", err)
	}
	log = logger.Sugar()
}
