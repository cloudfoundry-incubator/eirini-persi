// Package testing contains methods to create test data. It's a seaparate
// package to avoid import cycles. Helper functions can be found in the package
// `testhelper`.
package testing

import (
	"context"

	eirinix_catalog "code.cloudfoundry.org/eirinix/testing"
	testing_utils "code.cloudfoundry.org/quarks-utils/testing"
	corev1 "k8s.io/api/core/v1"
)

// NewCatalog returns a Catalog, our helper for test cases
func NewCatalog() Catalog {
	return Catalog{Catalog: &eirinix_catalog.Catalog{}}
}

// NewContext returns a non-nil empty context, for usage when it is unclear
// which context to use.  Mostly used in tests.
func NewContext() context.Context {
	return testing_utils.NewContext()
}

// Catalog provides several instances for test, based on the cf-operator's catalog
type Catalog struct{ *eirinix_catalog.Catalog }

// PodWithVcapServices generates a labeled pod with VCAP_SERVICES environment variable set
func (c *Catalog) PodWithVcapServices(name string, labels map[string]string, vcapServices string) corev1.Pod {

	pod := c.Catalog.LabeledPod(name, labels)
	pod.Spec.Containers[0].Env = []corev1.EnvVar{
		corev1.EnvVar{
			Name:  "VCAP_SERVICES",
			Value: vcapServices,
		}}

	return pod
}

// DefaultEiriniAppPod generates an Eirini Application pod with VCAP_SERVICES environment variable set
func (c *Catalog) DefaultEiriniAppPod(name string, vcapServices string) corev1.Pod {
	return c.PodWithVcapServices(name, map[string]string{"source_type": "APP"}, vcapServices)
}

// SimplePersiApp generates an Eirini Application pod which requires persistent volume (1 volume)
func (c *Catalog) SimplePersiApp(name string) corev1.Pod {
	return c.DefaultEiriniAppPod(name, `{"eirini-persi": [	  {
		"credentials": { "volume_id": "the-volume-id" },
		"label": "eirini-persi",
		"name": "my-instance",
		"plan": "hostpath",
		"tags": [
			"erini",
			"kubernetes",
			"storage"
		],
		"volume_mounts": [
		  {
			"container_dir": "/var/vcap/data/de847d34-bdcc-4c5d-92b1-cf2158a15b47",
			"device_type": "shared",
			"mode": "rw"
		  }
		]
	  }
	]
}`)
}

func (c *Catalog) MultipleVolumePersiAppOps() []string {
	return []string{
		`{"op":"add","path":"/spec/containers/0/volumeMounts","value":[{"mountPath":"/var/vcap/data/de847d34-bdcc-4c5d-92b1-cf2158a15b47","name":"the-volume-id1"},{"mountPath":"/var/vcap/data/de847d34-bdcc-4c5d-92b1-cf2158a15b47","name":"the-volume-id2"},{"mountPath":"/var/vcap/data/de847d34-bdcc-4c5d-92b1-cf2158a15b47","name":"the-volume-id3"}]}`,
		`{"op":"add","path":"/spec/volumes","value":[{"name":"the-volume-id1","persistentVolumeClaim":{"claimName":"the-volume-id1"}},{"name":"the-volume-id2","persistentVolumeClaim":{"claimName":"the-volume-id2"}},{"name":"the-volume-id3","persistentVolumeClaim":{"claimName":"the-volume-id3"}}]}`,
	}
}

// MultipleVolumePersiApp generates an Eirini Application pod which requires persistent volume (3 volumes)
func (c *Catalog) MultipleVolumePersiApp(name string) corev1.Pod {
	return c.DefaultEiriniAppPod(name, `{"eirini-persi": [	  {
		"credentials": { "volume_id": "the-volume-id1" },
		"label": "eirini-persi",
		"name": "my-instance",
		"plan": "hostpath",
		"tags": [
			"erini",
			"kubernetes",
			"storage"
		],
		"volume_mounts": [
			{
				"container_dir": "/var/vcap/data/de847d34-bdcc-4c5d-92b1-cf2158a15b47",
				"device_type": "shared",
				"mode": "rw"
			}
		]
	  },
	  {
		"credentials": { "volume_id": "the-volume-id2" },
		"label": "eirini-persi",
		"name": "my-instance",
		"plan": "hostpath",
		"tags": [
			"erini",
			"kubernetes",
			"storage"
		],
		"volume_mounts": [
			{
				"container_dir": "/var/vcap/data/de847d34-bdcc-4c5d-92b1-cf2158a15b47",
				"device_type": "shared",
				"mode": "rw"
			}
		]
	  },
	  {
		"credentials": { "volume_id": "the-volume-id3" },
		"label": "eirini-persi",
		"name": "my-instance",
		"plan": "hostpath",
		"tags": [
			"erini",
			"kubernetes",
			"storage"
		],
		"volume_mounts": [
			{
				"container_dir": "/var/vcap/data/de847d34-bdcc-4c5d-92b1-cf2158a15b47",
				"device_type": "shared",
				"mode": "rw"
			}
		]
	  }
	]
}`)
}
