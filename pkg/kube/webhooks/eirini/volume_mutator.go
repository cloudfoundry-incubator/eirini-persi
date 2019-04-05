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

// Device is the device which the volume is refered to
type Device struct {
	VolumeID string `json:"volume_id"` // VolumeID represents a Persistent Volume Claim
}

// VolumeMount is a volume assigned to the app
type VolumeMount struct {
	ContainerDir string `json:"container_dir"`
	DeviceType   string `json:"device_type"`
	Mode         string `json:"mode"`
	Device       Device `json:"device"`
}

// VcapService contains the service configuration. We look only at volume mounts here
type VcapService struct {
	VolumeMounts []VolumeMount `json:"volume_mounts"`
}

// VcapServices represent the VCAP_SERVICE structure, specific to this extension
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

// AppendMounts appends volumes that are specified in VCAP_SERVICES to the pod and to the container given as arguments
func (s VcapServices) AppendMounts(patchedPod *corev1.Pod, c *corev1.Container) {
	for _, volumeService := range s.ServiceMap {
		for _, volumeMount := range volumeService.VolumeMounts {
			if !containsContainerMount(c.VolumeMounts, volumeMount.Device.VolumeID) {
				patchedPod.Spec.Volumes = append(patchedPod.Spec.Volumes, corev1.Volume{
					Name: volumeMount.Device.VolumeID,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: volumeMount.Device.VolumeID,
						},
					},
				})

				c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
					Name:      volumeMount.Device.VolumeID,
					MountPath: volumeMount.ContainerDir,
				})
			}
		}
	}
}

// PatchPod alters the pod given as argument with the required volumes mounted
func (m *VolumeMutator) PatchPod(patchedPod *corev1.Pod) (*corev1.Pod, error) {
	for i := range patchedPod.Spec.Containers {
		c := &patchedPod.Spec.Containers[i]
		for _, e := range c.Env {
			if e.Name != "VCAP_SERVICES" {
				continue
			}
			var services VcapServices
			err := json.Unmarshal([]byte(e.Value), &services)
			if err != nil {
				return nil, err
			}
			services.AppendMounts(patchedPod, c)
			break
		}
	}
	return patchedPod, nil
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
	podCopy := pod.DeepCopy()

	// Patch only applications pod created by Eirini
	if v, ok := pod.GetLabels()["source_type"]; ok && v == "APP" {
		podCopy, err = m.PatchPod(podCopy)
		if err != nil {
			return admission.ErrorResponse(http.StatusBadRequest, err)
		}
	}

	return admission.PatchResponse(pod, podCopy)
}
