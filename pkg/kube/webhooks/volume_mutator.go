package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

// VolumeMount is a volume assigned to the app
type VolumeMount struct {
	ContainerDir string `json:"container_dir"`
	DeviceType   string `json:"device_type"`
	Mode         string `json:"mode"`
}

type Credentials struct {
	VolumeID string `json:"volume_id"` // VolumeID represents a Persistent Volume Claim
}

// VcapService contains the service configuration. We look only at volume mounts here
type VcapService struct {
	Credentials  Credentials   `json:"credentials"`
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
			if !containsContainerMount(c.VolumeMounts, volumeService.Credentials.VolumeID) {
				patchedPod.Spec.Volumes = append(patchedPod.Spec.Volumes, corev1.Volume{
					Name: volumeService.Credentials.VolumeID,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: volumeService.Credentials.VolumeID,
						},
					},
				})

				c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
					Name:      volumeService.Credentials.VolumeID,
					MountPath: volumeMount.ContainerDir,
				})
				u := int64(0)
				patchedPod.Spec.InitContainers = []corev1.Container{{
					SecurityContext: &corev1.SecurityContext{RunAsUser: &u},
					Name:            "eirini-persi",
					Image:           c.Image,
					VolumeMounts:    c.VolumeMounts,
					Command: []string{
						"sh",
						"-c",
						fmt.Sprintf("chown -R vcap:vcap %s", volumeMount.ContainerDir),
					},
				}}
			}
		}
	}
}

// MountVcapVolumes alters the pod given as argument with the required volumes mounted
func (m *VolumeMutator) MountVcapVolumes(patchedPod *corev1.Pod) error {
	for i := range patchedPod.Spec.Containers {
		c := &patchedPod.Spec.Containers[i]
		for _, e := range c.Env {
			if e.Name != "VCAP_SERVICES" {
				continue
			}
			m.log.Debug("Appending volumes to the Eirini App")

			var services VcapServices
			err := json.Unmarshal([]byte(e.Value), &services)
			if err != nil {
				return err
			}
			services.AppendMounts(patchedPod, c)
			break
		}
	}
	return nil
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
	m.log.Debugf("Handling webhook request for POD: %s (%s)", podCopy.Name, podCopy.Namespace)

	// Patch only applications pod created by Eirini
	if v, ok := pod.GetLabels()["source_type"]; ok && v == "APP" {

		err = m.MountVcapVolumes(podCopy)
		if err != nil {
			return admission.ErrorResponse(http.StatusBadRequest, err)
		}
	}

	return admission.PatchResponse(pod, podCopy)
}

// VolumeMutator implements inject.Client.
// A client will be automatically injected.
var _ inject.Client = &VolumeMutator{}

// InjectClient injects the client.
func (m *VolumeMutator) InjectClient(c client.Client) error {
	m.client = c
	return nil
}

// VolumeMutator implements inject.Decoder.
// A decoder will be automatically injected.
var _ inject.Decoder = &VolumeMutator{}

// InjectDecoder injects the decoder.
func (m *VolumeMutator) InjectDecoder(d types.Decoder) error {
	m.decoder = d
	return nil
}
