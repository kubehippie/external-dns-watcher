/*
Copyright 2025 Thomas Boerger <thomas@webhippie.de>.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1alpha1 "github.com/kubehippie/external-dns-watcher/api/v1alpha1"
	webhookv1alpha1 "github.com/kubehippie/external-dns-watcher/internal/webhook/v1alpha1"
)

const testStatusIPPath = "$.status.ip"

var _ = Describe("DNSWatcherCustomDefaulter", func() {
	var (
		ctx       context.Context
		defaulter *webhookv1alpha1.DNSWatcherCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = &webhookv1alpha1.DNSWatcherCustomDefaulter{}
	})

	It("normalises lowercase DNS types to uppercase", func() {
		w := &apiv1alpha1.DNSWatcher{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: apiv1alpha1.DNSWatcherSpec{
				Paths: []apiv1alpha1.PathSpec{
					{Path: testStatusIPPath, Type: "a"},
					{Path: "$.status.ipv6", Type: "aaaa"},
					{Path: "$.status.host", Type: "cname"},
				},
			},
		}
		Expect(defaulter.Default(ctx, w)).To(Succeed())
		Expect(w.Spec.Paths[0].Type).To(Equal("A"))
		Expect(w.Spec.Paths[1].Type).To(Equal("AAAA"))
		Expect(w.Spec.Paths[2].Type).To(Equal("CNAME"))
	})

	It("leaves already-uppercase types unchanged", func() {
		w := &apiv1alpha1.DNSWatcher{
			Spec: apiv1alpha1.DNSWatcherSpec{
				Paths: []apiv1alpha1.PathSpec{
					{Path: testStatusIPPath, Type: "A"},
				},
			},
		}
		Expect(defaulter.Default(ctx, w)).To(Succeed())
		Expect(w.Spec.Paths[0].Type).To(Equal("A"))
	})

	It("handles an empty path list without error", func() {
		w := &apiv1alpha1.DNSWatcher{Spec: apiv1alpha1.DNSWatcherSpec{}}
		Expect(defaulter.Default(ctx, w)).To(Succeed())
	})
})

var _ = Describe("DNSWatcherCustomValidator", func() {
	var (
		ctx       context.Context
		validator *webhookv1alpha1.DNSWatcherCustomValidator
	)

	BeforeEach(func() {
		ctx = context.Background()
		validator = &webhookv1alpha1.DNSWatcherCustomValidator{}
	})

	validWatcher := func(overrides ...func(*apiv1alpha1.DNSWatcher)) *apiv1alpha1.DNSWatcher {
		w := &apiv1alpha1.DNSWatcher{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: apiv1alpha1.DNSWatcherSpec{
				Group:          "infrastructure.cluster.x-k8s.io",
				Version:        "v1beta1",
				Kind:           "HetznerCluster",
				RecordTemplate: "{{ .Name }}.example.com",
				Paths: []apiv1alpha1.PathSpec{
					{Path: testStatusIPPath, Type: "A"},
				},
			},
		}
		for _, fn := range overrides {
			fn(w)
		}
		return w
	}

	Context("ValidateCreate", func() {
		It("accepts a fully valid spec", func() {
			_, err := validator.ValidateCreate(ctx, validWatcher())
			Expect(err).NotTo(HaveOccurred())
		})

		It("accepts empty group for core Kubernetes resources", func() {
			w := validWatcher(func(w *apiv1alpha1.DNSWatcher) { w.Spec.Group = "" })
			_, err := validator.ValidateCreate(ctx, w)
			Expect(err).NotTo(HaveOccurred())
		})

		It("rejects empty version", func() {
			w := validWatcher(func(w *apiv1alpha1.DNSWatcher) { w.Spec.Version = "" })
			_, err := validator.ValidateCreate(ctx, w)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("version"))
		})

		It("rejects empty kind", func() {
			w := validWatcher(func(w *apiv1alpha1.DNSWatcher) { w.Spec.Kind = "" })
			_, err := validator.ValidateCreate(ctx, w)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("kind"))
		})

		It("rejects empty recordTemplate", func() {
			w := validWatcher(func(w *apiv1alpha1.DNSWatcher) { w.Spec.RecordTemplate = "" })
			_, err := validator.ValidateCreate(ctx, w)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("recordTemplate"))
		})

		It("rejects invalid Go template syntax", func() {
			w := validWatcher(func(w *apiv1alpha1.DNSWatcher) { w.Spec.RecordTemplate = "{{ .Name" })
			_, err := validator.ValidateCreate(ctx, w)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("recordTemplate"))
		})

		It("rejects template with unknown variables", func() {
			w := validWatcher(func(w *apiv1alpha1.DNSWatcher) { w.Spec.RecordTemplate = "{{ .UnknownField }}.example.com" })
			_, err := validator.ValidateCreate(ctx, w)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("recordTemplate"))
		})

		It("rejects empty paths", func() {
			w := validWatcher(func(w *apiv1alpha1.DNSWatcher) { w.Spec.Paths = nil })
			_, err := validator.ValidateCreate(ctx, w)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("paths"))
		})

		It("rejects path without leading $", func() {
			w := validWatcher(func(w *apiv1alpha1.DNSWatcher) {
				w.Spec.Paths = []apiv1alpha1.PathSpec{{Path: "status.ip", Type: "A"}}
			})
			_, err := validator.ValidateCreate(ctx, w)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path"))
		})

		It("rejects empty path", func() {
			w := validWatcher(func(w *apiv1alpha1.DNSWatcher) {
				w.Spec.Paths = []apiv1alpha1.PathSpec{{Path: "", Type: "A"}}
			})
			_, err := validator.ValidateCreate(ctx, w)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path"))
		})

		It("rejects invalid DNS type", func() {
			w := validWatcher(func(w *apiv1alpha1.DNSWatcher) {
				w.Spec.Paths = []apiv1alpha1.PathSpec{{Path: testStatusIPPath, Type: "INVALID"}}
			})
			_, err := validator.ValidateCreate(ctx, w)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("type"))
		})

		It("accepts all valid DNS types", func() {
			for _, t := range []string{"A", "AAAA", "CNAME", "TXT", "MX", "NS", "SRV", "PTR", "SOA", "CAA", "NAPTR"} {
				w := validWatcher(func(w *apiv1alpha1.DNSWatcher) {
					w.Spec.Paths = []apiv1alpha1.PathSpec{{Path: testStatusIPPath, Type: t}}
				})
				_, err := validator.ValidateCreate(ctx, w)
				Expect(err).NotTo(HaveOccurred(), "type %s should be valid", t)
			}
		})
	})

	Context("ValidateUpdate", func() {
		It("validates the updated spec", func() {
			old := validWatcher()
			updated := validWatcher(func(w *apiv1alpha1.DNSWatcher) { w.Spec.Kind = "" })
			_, err := validator.ValidateUpdate(ctx, old, updated)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("kind"))
		})
	})

	Context("ValidateDelete", func() {
		It("always allows deletion", func() {
			_, err := validator.ValidateDelete(ctx, validWatcher())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
