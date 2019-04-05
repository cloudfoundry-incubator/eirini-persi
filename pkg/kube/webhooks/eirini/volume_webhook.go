package eirini

import (
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/builder"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

type GetPodFuncType func(types.Decoder, types.Request) (*corev1.Pod, error)

func GetPodFunc(decoder types.Decoder, req types.Request) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	err := decoder.Decode(req, pod)
	return pod, err
}

// Volume is a webhook for mounting persistent volumes in eirini application pods
func Volume(log *zap.SugaredLogger, config *config.Config, mgr manager.Manager, hookServer *webhook.Server) (*admission.Webhook, error) {
	log.Info("Creating the eirini volume webhook")

	volumeMutator := NewVolumeMutator(log, config, mgr, controllerutil.SetControllerReference, GetPodFunc)

	mutatingWebhook, err := builder.NewWebhookBuilder().
		Path("/volume").
		Mutating().
		NamespaceSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"cf-operator-ns": config.Namespace,
			},
		}).
		ForType(&corev1.Pod{}).
		Handlers(volumeMutator).
		WithManager(mgr).
		FailurePolicy(admissionregistrationv1beta1.Fail).
		Build()

	if err != nil {
		return nil, errors.Wrap(err, "couldn't build a new webhook")
	}

	err = hookServer.Register(mutatingWebhook)
	if err != nil {
		return nil, errors.Wrap(err, "unable to register the hook with the admission server")
	}

	return mutatingWebhook, nil
}
