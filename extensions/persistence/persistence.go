package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"

	eirinix "code.cloudfoundry.org/eirinix"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// VolumeMount is a volume assigned to the app
type VolumeMount struct {
	ContainerDir string `json:"container_dir"`
	DeviceType   string `json:"device_type"`
	Mode         string `json:"mode"`
}

// Credentials is containing the volume id assigned to the pod
type Credentials struct {
	// VolumeID represents a Persistent Volume Claim
	VolumeID string `json:"volume_id"`
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

// Extension changes pod definitions
type Extension struct{ Logger *zap.SugaredLogger }

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

				vcap := int64(2000) // Best guess for vcap group id
				if patchedPod.Spec.SecurityContext == nil {
					patchedPod.Spec.SecurityContext = &corev1.PodSecurityContext{
						RunAsUser:  &vcap,
						RunAsGroup: &vcap,
						FSGroup:    &vcap,
					}
				} else {
					// Try to find a better guess for the group id
					if patchedPod.Spec.SecurityContext.RunAsGroup != nil {
						vcap = *patchedPod.Spec.SecurityContext.RunAsGroup
					} else if patchedPod.Spec.SecurityContext.RunAsUser != nil {
						// Normally uid == gid for vcap user
						vcap = *patchedPod.Spec.SecurityContext.RunAsUser
					}
					if patchedPod.Spec.SecurityContext.FSGroup == nil {
						patchedPod.Spec.SecurityContext.FSGroup = &vcap
					}
					if patchedPod.Spec.SecurityContext.RunAsGroup == nil {
						patchedPod.Spec.SecurityContext.RunAsGroup = &vcap
					}
				}
			}
		}
	}
}

// MountVcapVolumes alters the pod given as argument with the required volumes mounted
func (ext *Extension) MountVcapVolumes(patchedPod *corev1.Pod) error {
	for i := range patchedPod.Spec.Containers {
		c := &patchedPod.Spec.Containers[i]
		for _, env := range c.Env {
			if env.Name != "VCAP_SERVICES" {
				continue
			}
			ext.Logger.Debug("Appending volumes to the Eirini App")

			var services VcapServices
			err := json.Unmarshal([]byte(env.Value), &services)
			if err != nil {
				return err
			}
			services.AppendMounts(patchedPod, c)
			break
		}
	}
	return nil
}

// New returns the persi extension
func New() eirinix.Extension {
	return &Extension{}
}

// Handle manages volume claims for ExtendedStatefulSet pods
func (ext *Extension) Handle(ctx context.Context, eiriniManager eirinix.Manager, pod *corev1.Pod, req admission.Request) admission.Response {

	if pod == nil {
		return admission.Errored(http.StatusBadRequest, errors.New("No pod could be decoded from the request"))
	}

	_, file, _, _ := runtime.Caller(0)
	log := eiriniManager.GetLogger().Named(file)

	ext.Logger = log
	podCopy := pod.DeepCopy()
	log.Debugf("Handling webhook request for POD: %s (%s)", podCopy.Name, podCopy.Namespace)

	err := ext.MountVcapVolumes(podCopy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	return eiriniManager.PatchFromPod(req, podCopy)
}
