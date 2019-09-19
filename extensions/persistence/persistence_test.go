package persistence_test

import (
	"context"
	"fmt"
	"net/http"

	persistence "github.com/SUSE/eirini-persi/extensions/persistence"
	eirinix "github.com/SUSE/eirinix"
	eirinixcatalog "github.com/SUSE/eirinix/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cfakes "github.com/SUSE/eirini-persi/pkg/controllers/fakes"
	"github.com/SUSE/eirini-persi/testing"
)

func decodePatches(resp admission.Response) string {
	var r string
	for _, patch := range resp.Patches {
		r += patch.Json()
	}
	return r
}

func ExpectInitContainer(pod *corev1.Pod, howmany int) {
	Expect(len(pod.Spec.InitContainers)).To(Equal(howmany))
	Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(howmany))

	for i, vm := range pod.Spec.Containers[0].VolumeMounts {

		Expect(pod.Spec.InitContainers[i].Name).To(Equal(fmt.Sprintf("eirini-persi-%s", vm.Name)))
		Expect(pod.Spec.InitContainers[i].Image).To(Equal(pod.Spec.Containers[0].Image))
		Expect(pod.Spec.InitContainers[i].Command).To(Equal([]string{
			"sh",
			"-c",
			fmt.Sprintf("chown -R vcap:vcap %s", vm.MountPath),
		}))
		Expect(*pod.Spec.InitContainers[i].SecurityContext.RunAsUser).To(Equal(int64(0)))
	}
}

var _ = Describe("Persistence Extension", func() {
	var (
		eirinixcat    eirinixcatalog.Catalog
		eiriniManager eirinix.Manager
		eiriniExt     eirinix.Extension
		manager       *cfakes.FakeManager
		client        *cfakes.FakeClient
		ctx           context.Context
		env           testing.Catalog
		request       admission.Request
	)

	BeforeEach(func() {
		client = &cfakes.FakeClient{}
		restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
		restMapper.Add(schema.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"}, meta.RESTScopeNamespace)

		manager = &cfakes.FakeManager{}
		//	manager.GetSchemeReturns(scheme.Scheme)
		manager.GetClientReturns(client)
		manager.GetRESTMapperReturns(restMapper)

		ctx = testing.NewContext()
		eirinixcat = eirinixcatalog.NewCatalog()
		eiriniManager = eirinixcat.SimpleManager()
		eiriniExt = persistence.New()
		request = admission.Request{AdmissionRequest: admissionv1beta1.AdmissionRequest{}}
	})

	Describe("Handle", func() {
		It("passes on errors from the decoding step", func() {
			ext := persistence.New()

			res := ext.Handle(ctx, eiriniManager, nil, request)
			Expect(res.AdmissionResponse.Result.Code).To(Equal(int32(http.StatusBadRequest)))
		})

		It("does not act if the source_type: APP label is not set", func() {
			pod := env.DefaultEiriniAppPod("foo", ``)

			resp := eiriniExt.Handle(ctx, eiriniManager, &pod, request)
			Expect(len(resp.Patches)).To(Equal(0))
		})

		It("does not with a no services app", func() {
			pod := env.DefaultEiriniAppPod("foo", `{}`)
			resp := eiriniExt.Handle(ctx, eiriniManager, &pod, request)
			Expect(len(resp.Patches)).To(Equal(0))
		})

		It("does act if the source_type: APP label is set and one volume is supplied", func() {
			pod := env.SimplePersiApp("foo")
			resp := eiriniExt.Handle(ctx, eiriniManager, &pod, request)
			Expect(len(resp.Patches)).To(Equal(3))
		})

		It("does act if the source_type: APP label is set and 3 volumes are supplied", func() {
			pod := env.MultipleVolumePersiApp("foo")

			resp := eiriniExt.Handle(ctx, eiriniManager, &pod, request)
			Expect(len(resp.Patches)).To(Equal(3))

			ops := env.MultipleVolumePersiAppOps()
			Expect(len(resp.Patches)).To(Equal(len(ops)))
			for _, op := range ops {
				Expect(decodePatches(resp)).Should(ContainSubstring(op))
			}
		})
	})

	Describe("AppendMounts", func() {
		It("append mounts if are existing", func() {
			var services persistence.VcapServices
			pod := env.DefaultEiriniAppPod("bar", ``)
			services.ServiceMap = append(services.ServiceMap, persistence.VcapService{
				Credentials:  persistence.Credentials{VolumeID: "foo"},
				VolumeMounts: []persistence.VolumeMount{persistence.VolumeMount{ContainerDir: "/foo/"}},
			})
			services.AppendMounts(&pod, &pod.Spec.Containers[0])

			Expect(pod.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("foo"))
			Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/foo/"))
			Expect(pod.Spec.Volumes[0].Name).To(Equal("foo"))
			Expect(pod.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal("foo"))
			ExpectInitContainer(&pod, 1)
		})

		It("is idempotent and does not append already existing mounts", func() {
			var services persistence.VcapServices
			pod := env.DefaultEiriniAppPod("bar", ``)
			services.ServiceMap = append(services.ServiceMap, persistence.VcapService{
				Credentials:  persistence.Credentials{VolumeID: "foo"},
				VolumeMounts: []persistence.VolumeMount{persistence.VolumeMount{ContainerDir: "/foo/"}},
			})

			Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(0))
			Expect(len(pod.Spec.Volumes)).To(Equal(0))
			services.AppendMounts(&pod, &pod.Spec.Containers[0])
			Expect(len(pod.Spec.Volumes)).To(Equal(1))
			Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(1))
			services.AppendMounts(&pod, &pod.Spec.Containers[0])
			Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(1))
			Expect(len(pod.Spec.Volumes)).To(Equal(1))
		})
	})

	Describe("MountVcapVolumes", func() {
		It("append mounts if pods declare them in VCAP_SERVICES", func() {
			pod := env.MultipleVolumePersiApp("foo")
			ext, ok := eiriniExt.(*persistence.Extension)
			Expect(ok).To(BeTrue())
			ext.Logger = eiriniManager.GetLogger()

			err := ext.MountVcapVolumes(&pod)

			Expect(err).ToNot(HaveOccurred())
			Expect(pod.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("the-volume-id1"))
			Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/data/de847d34-bdcc-4c5d-92b1-cf2158a15b47"))
			Expect(pod.Spec.Volumes[0].Name).To(Equal("the-volume-id1"))
			Expect(pod.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal("the-volume-id1"))
			ExpectInitContainer(&pod, 3)
		})

		It("does nothing if env is empty", func() {
			pod := env.DefaultEiriniAppPod("foo", `{}`)
			ext, ok := eiriniExt.(*persistence.Extension)
			Expect(ok).To(BeTrue())
			ext.Logger = eiriniManager.GetLogger()

			err := ext.MountVcapVolumes(&pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(0))
			Expect(len(pod.Spec.Volumes)).To(Equal(0))
		})

		It("returns an error if VCAP_SERVICES is not a json", func() {
			pod := env.DefaultEiriniAppPod("foo", ``)
			ext, ok := eiriniExt.(*persistence.Extension)
			Expect(ok).To(BeTrue())
			ext.Logger = eiriniManager.GetLogger()
			err := ext.MountVcapVolumes(&pod)
			Expect(err).To(HaveOccurred())
		})
	})

})
