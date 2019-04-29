package controllers

import (
	"context"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	webhooks "github.com/SUSE/eirini-extensions/pkg/kube/webhooks"
	"github.com/SUSE/eirini-extensions/pkg/util/ctxlog"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	machinerytypes "k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var addToManagerFuncs = []func(context.Context, *config.Config, manager.Manager) error{}

var addToSchemes = runtime.SchemeBuilder{}

var addHookFuncs = []func(*zap.SugaredLogger, *config.Config, manager.Manager, *webhook.Server) (*admission.Webhook, error){
	webhooks.Volume,
}

// AddToManager adds all Controllers to the Manager
func AddToManager(ctx context.Context, config *config.Config, m manager.Manager) error {
	for _, f := range addToManagerFuncs {
		if err := f(ctx, config, m); err != nil {
			return err
		}
	}
	return nil
}

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return addToSchemes.AddToScheme(s)
}

func CreateOrGetClusterIP(ctx context.Context, config *config.Config, m manager.Manager) (string, error) {
	serviceName := "eirini-extensions-webhook-server"

	client, err := corev1client.NewForConfig(m.GetConfig())
	if err != nil {
		return "", errors.Wrap(err, "Could not get kube client")
	}
	services := client.Services(config.Namespace)

	createdService, err := services.Create(
		&v1.Service{
			//TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: serviceName},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeClusterIP,
				Ports: []v1.ServicePort{
					v1.ServicePort{Name: "eirini-extensions-operator", Port: config.WebhookServerPort, Protocol: v1.ProtocolTCP},
				},
			},
		})
	if err != nil {
		ctxlog.Errorf(ctx, "Failed creating service: %s", err.Error())

		// Try to get cluster ip if already existing, otherwise bail out
		existingService, err := services.Get(serviceName, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return existingService.Spec.ClusterIP, nil
	}
	return createdService.Spec.ClusterIP, nil
}

// AddHooks adds all web hooks to the Manager
func AddHooks(ctx context.Context, config *config.Config, m manager.Manager, generator credsgen.Generator) error {
	ctxlog.Infof(ctx, "Setting up webhook server on %s:%d", config.WebhookServerHost, config.WebhookServerPort)

	webhookConfig := NewWebhookConfig(m.GetClient(), config, generator, "eirini-extensions-mutating-hook-"+config.Namespace)

	if config.WebhookServerHost == "" {
		ip, err := CreateOrGetClusterIP(ctx, config, m)
		if err != nil {
			return errors.Wrap(err, "unable to find bind ip address")
		}
		ctxlog.Infof(ctx, "Binding to ClusterIP: %s", ip)
		config.WebhookServerHost = ip
	}

	disableConfigInstaller := true
	hookServer, err := webhook.NewServer("eirini-extensions", m, webhook.ServerOptions{
		Port:    config.WebhookServerPort,
		CertDir: webhookConfig.CertDir,
		DisableWebhookConfigInstaller: &disableConfigInstaller,
		BootstrapOptions: &webhook.BootstrapOptions{
			MutatingWebhookConfigName: webhookConfig.ConfigName,
			Host: &config.WebhookServerHost,
			// The user should probably be able to use a service instead.
			// Service: ??
		},
	})

	if err != nil {
		return errors.Wrap(err, "unable to create a new webhook server")
	}

	log := ctxlog.ExtractLogger(ctx)
	webhooks := []*admission.Webhook{}
	for _, f := range addHookFuncs {
		wh, err := f(log, config, m, hookServer)
		if err != nil {
			return err
		}
		webhooks = append(webhooks, wh)
	}

	err = setOperatorNamespaceLabel(ctx, config, m.GetClient())
	if err != nil {
		return errors.Wrap(err, "setting the operator namespace label")
	}

	err = webhookConfig.setupCertificate(ctx)
	if err != nil {
		return errors.Wrap(err, "setting up the webhook server certificate")
	}
	err = webhookConfig.generateWebhookServerConfig(ctx, webhooks)
	if err != nil {
		return errors.Wrap(err, "generating the webhook server configuration")
	}

	return err
}

func setOperatorNamespaceLabel(ctx context.Context, config *config.Config, c client.Client) error {
	ns := &unstructured.Unstructured{}
	ns.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Kind:    "Namespace",
		Version: "v1",
	})
	err := c.Get(ctx, machinerytypes.NamespacedName{Name: config.Namespace}, ns)

	if err != nil {
		return errors.Wrap(err, "getting the namespace object")
	}

	labels := ns.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["eirini-extensions-ns"] = config.Namespace
	ns.SetLabels(labels)
	err = c.Update(ctx, ns)

	if err != nil {
		return errors.Wrap(err, "updating the namespace object")
	}

	return nil
}
