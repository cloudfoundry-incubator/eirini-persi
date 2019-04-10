package webhooks_test

import (
	"context"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"

	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned/scheme"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
	webhooks "github.com/SUSE/eirini-extensions/pkg/kube/webhooks"
	"github.com/SUSE/eirini-extensions/testing"
)

func generateGetPodFunc(pod *corev1.Pod, err error) webhooks.GetPodFuncType {
	return func(_ types.Decoder, _ types.Request) (*corev1.Pod, error) {
		return pod, err
	}
}

var _ = Describe("Volume Mutator", func() {

	var (
		manager          *cfakes.FakeManager
		client           *cfakes.FakeClient
		ctx              context.Context
		config           *config.Config
		env              testing.Catalog
		log              *zap.SugaredLogger
		request          types.Request
		setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error = func(owner, object metav1.Object, scheme *runtime.Scheme) error { return nil }
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		client = &cfakes.FakeClient{}
		restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
		restMapper.Add(schema.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"}, meta.RESTScopeNamespace)

		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)
		manager.GetClientReturns(client)
		manager.GetRESTMapperReturns(restMapper)

		config = env.DefaultConfig()
		ctx = testing.NewContext()
		_, log = helper.NewTestLogger()

		request = types.Request{AdmissionRequest: &admissionv1beta1.AdmissionRequest{}}
	})

	Describe("Handle", func() {
		It("passes on errors from the decoding step", func() {
			f := generateGetPodFunc(nil, fmt.Errorf("decode failed"))
			mutator := webhooks.NewVolumeMutator(log, config, manager, setReferenceFunc, f)

			res := mutator.Handle(ctx, request)
			Expect(res.Response.Result.Code).To(Equal(int32(http.StatusBadRequest)))
		})

		It("does not act if the source_type: APP label is not set", func() {
			pod := env.DefaultEiriniAppPod("foo", ``)
			f := generateGetPodFunc(&pod, nil)

			mutator := webhooks.NewVolumeMutator(log, config, manager, setReferenceFunc, f)
			resp := mutator.Handle(ctx, request)
			Expect(len(resp.Patches)).To(Equal(0))
		})

		It("does not with a no services app", func() {
			pod := env.DefaultEiriniAppPod("foo", `{}`)
			f := generateGetPodFunc(&pod, nil)

			mutator := webhooks.NewVolumeMutator(log, config, manager, setReferenceFunc, f)

			resp := mutator.Handle(ctx, request)
			Expect(len(resp.Patches)).To(Equal(0))
		})

		It("does act if the source_type: APP label is set and one volume is supplied", func() {
			pod := env.SimplePersiApp("foo")
			f := generateGetPodFunc(&pod, nil)

			mutator := webhooks.NewVolumeMutator(log, config, manager, setReferenceFunc, f)
			resp := mutator.Handle(ctx, request)

			Expect(len(resp.Patches)).To(Equal(2))
		})

		It("does act if the source_type: APP label is set and 3 volumes are supplied", func() {
			pod := env.MultipleVolumePersiApp("foo")
			f := generateGetPodFunc(&pod, nil)
			mutator := webhooks.NewVolumeMutator(log, config, manager, setReferenceFunc, f)
			resp := mutator.Handle(ctx, request)

			Expect(len(resp.Patches)).To(Equal(2))
			Expect(len(resp.Patches[0].Value.([]interface{}))).To(Equal(3))
			Expect(len(resp.Patches[1].Value.([]interface{}))).To(Equal(3))
		})
	})

	Describe("AppendMounts", func() {
		It("append mounts if are not existing", func() {
			var services webhooks.VcapServices
			pod := env.DefaultEiriniAppPod("bar", ``)
			services.ServiceMap = append(services.ServiceMap, webhooks.VcapService{VolumeMounts: []webhooks.VolumeMount{webhooks.VolumeMount{ContainerDir: "/foo/", Device: webhooks.Device{VolumeID: "foo"}}}})
			services.AppendMounts(&pod, &pod.Spec.Containers[0])

			Expect(pod.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("foo"))
			Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/foo/"))
			Expect(pod.Spec.Volumes[0].Name).To(Equal("foo"))
			Expect(pod.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal("foo"))
		})

		It("is idempotent and does not append already existing mounts", func() {
			var services webhooks.VcapServices
			pod := env.DefaultEiriniAppPod("bar", ``)
			services.ServiceMap = append(services.ServiceMap, webhooks.VcapService{VolumeMounts: []webhooks.VolumeMount{webhooks.VolumeMount{ContainerDir: "/foo/", Device: webhooks.Device{VolumeID: "foo"}}}})

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
			f := generateGetPodFunc(&pod, nil)
			mutator := webhooks.NewVolumeMutator(log, config, manager, setReferenceFunc, f)

			volumeMutator, ok := mutator.(*webhooks.VolumeMutator)
			Expect(ok).To(BeTrue())

			err := volumeMutator.MountVcapVolumes(&pod)

			Expect(err).ToNot(HaveOccurred())
			Expect(pod.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("the-volume-id1"))
			Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/data/de847d34-bdcc-4c5d-92b1-cf2158a15b47"))
			Expect(pod.Spec.Volumes[0].Name).To(Equal("the-volume-id1"))
			Expect(pod.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal("the-volume-id1"))
		})

		It("does nothing if env is empty", func() {
			pod := env.DefaultEiriniAppPod("foo", `{}`)
			f := generateGetPodFunc(&pod, nil)
			mutator := webhooks.NewVolumeMutator(log, config, manager, setReferenceFunc, f)
			volumeMutator, ok := mutator.(*webhooks.VolumeMutator)
			Expect(ok).To(BeTrue())

			err := volumeMutator.MountVcapVolumes(&pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(0))
			Expect(len(pod.Spec.Volumes)).To(Equal(0))
		})

		It("returns an error if VCAP_SERVICES is not a json", func() {
			pod := env.DefaultEiriniAppPod("foo", ``)
			f := generateGetPodFunc(&pod, nil)
			mutator := webhooks.NewVolumeMutator(log, config, manager, setReferenceFunc, f)
			volumeMutator, ok := mutator.(*webhooks.VolumeMutator)
			Expect(ok).To(BeTrue())

			err := volumeMutator.MountVcapVolumes(&pod)
			Expect(err).To(HaveOccurred())
		})
	})

})
