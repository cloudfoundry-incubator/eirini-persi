package eirini

import (
	"context"
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

type Device struct {
	VolumeId string `json:"volume_id"` // VolumeId represents a Persistent Volume Claim
}

type VolumeMount struct {
	ContainerDir string `json:"container_dir"`
	DeviceType   string `json:"device_type"`
	Mode         string `json:"mode"`
	Device       Device `json:"device"`
}

type VcapService struct {
	VolumeMounts []VolumeMount `json:"volume_mounts"`
}

type VcapServices struct {
	ServiceMap []VcapService `json:"eirini-persi"`
}

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// VolumeMutator changes pod definitions
type VolumeMutator struct {
	client       client.Client
	scheme       *runtime.Scheme
	setReference setReferenceFunc
	log          *zap.SugaredLogger
	config       *config.Config
	decoder      types.Decoder
	getPodFunc   GetPodFuncType
}

// Implement admission.Handler so the controller can handle admission request.
var _ admission.Handler = &VolumeMutator{}

func containsContainerMount(containermounts []corev1.VolumeMount, mount string) bool {
	for _, m := range containermounts {
		if m.Name == mount {
			return true
		}
	}
	return false
}

// NewVolumeMutator returns a new reconcile.Reconciler
func NewVolumeMutator(log *zap.SugaredLogger, config *config.Config, mgr manager.Manager, srf setReferenceFunc, getPodFunc GetPodFuncType) admission.Handler {
	mutatorLog := log.Named("eirini-volume-mutator")
	mutatorLog.Info("Creating a Volume mutator")

	return &VolumeMutator{
		log:          mutatorLog,
		config:       config,
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		setReference: srf,
		getPodFunc:   getPodFunc,
	}
}

// Handle manages volume claims for ExtendedStatefulSet pods
func (m *VolumeMutator) Handle(ctx context.Context, req types.Request) types.Response {
	pod, err := m.getPodFunc(m.decoder, req)
	if err != nil {
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}
	patchedPod := pod.DeepCopy()

	if v, ok := pod.GetLabels()["source_type"]; ok && v == "APP" {
		var services VcapServices
		for i := range patchedPod.Spec.Containers {
			c := &patchedPod.Spec.Containers[i]
			for _, e := range c.Env {
				if e.Name == "VCAP_SERVICES" {
					err := json.Unmarshal([]byte(e.Value), &services)
					if err != nil {
						return admission.ErrorResponse(http.StatusBadRequest, err)
					}
					for _, volumeService := range services.ServiceMap {
						for _, volumeMount := range volumeService.VolumeMounts {
							if !containsContainerMount(c.VolumeMounts, volumeMount.Device.VolumeId) {
								patchedPod.Spec.Volumes = append(patchedPod.Spec.Volumes, corev1.Volume{
									Name: volumeMount.Device.VolumeId,
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: volumeMount.Device.VolumeId,
										},
									},
								})

								c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
									Name:      volumeMount.Device.VolumeId,
									MountPath: volumeMount.ContainerDir,
								})
							}
						}
					}

					break
				}
			}
		}
	}
	return admission.PatchResponse(pod, patchedPod)
}
