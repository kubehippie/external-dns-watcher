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

package controller_test

import (
	"context"

	"github.com/kubehippie/external-dns-watcher/internal/controller"
	"github.com/kubehippie/external-dns-watcher/pkg/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	extdnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

const (
	testEndpointRecordTemplate = "{{ .Name }}.static.example.com"
	testEndpointIPPath         = "$.data.ip"
	testIPv4                   = "1.2.3.4"
)

var _ = Describe("EndpointReconciler", func() {
	var (
		ctx context.Context
		ns  *corev1.Namespace
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create a unique namespace per test for isolation.
		ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "endpoint-"}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), ns,
			client.PropagationPolicy(metav1.DeletePropagationForeground),
		)).To(Succeed())
	})

	Context("when reconciling a watched ConfigMap", func() {
		It("should create a DNSEndpoint named after the target resource", func() {
			reconciler := &controller.EndpointReconciler{
				Client: k8sClient,
				Scheme: scheme,
				WatchConfigs: []config.WatchConfig{
					{
						Group:          "",
						Version:        "v1",
						Kind:           testConfigMapKind,
						RecordTemplate: testEndpointRecordTemplate,
						Paths: []config.PathConfig{
							{Path: testEndpointIPPath, Type: "A"},
						},
					},
				},
			}

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-resource",
					Namespace: ns.Name,
				},
				Data: map[string]string{"ip": testIPv4},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			ep := &extdnsv1alpha1.DNSEndpoint{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: ns.Name}, ep)).To(Succeed())
			Expect(ep.Spec.Endpoints).To(HaveLen(1))
			Expect(ep.Spec.Endpoints[0].DNSName).To(Equal("my-resource.static.example.com"))
			Expect(ep.Spec.Endpoints[0].RecordType).To(Equal("A"))
			Expect(ep.Spec.Endpoints[0].Targets).To(ConsistOf(testIPv4))
		})

		It("should update the DNSEndpoint when the target resource changes", func() {
			reconciler := &controller.EndpointReconciler{
				Client: k8sClient,
				Scheme: scheme,
				WatchConfigs: []config.WatchConfig{
					{
						Group:          "",
						Version:        "v1",
						Kind:           testConfigMapKind,
						RecordTemplate: testEndpointRecordTemplate,
						Paths: []config.PathConfig{
							{Path: testEndpointIPPath, Type: "A"},
						},
					},
				},
			}

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "changing-resource",
					Namespace: ns.Name,
				},
				Data: map[string]string{"ip": "10.0.0.1"},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}}
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			epKey := types.NamespacedName{Name: cm.Name, Namespace: ns.Name}
			ep := &extdnsv1alpha1.DNSEndpoint{}
			Expect(k8sClient.Get(ctx, epKey, ep)).To(Succeed())
			Expect(ep.Spec.Endpoints[0].Targets).To(ConsistOf("10.0.0.1"))

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
			cm.Data["ip"] = "10.0.0.2"
			Expect(k8sClient.Update(ctx, cm)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, epKey, ep)).To(Succeed())
			Expect(ep.Spec.Endpoints[0].Targets).To(ConsistOf("10.0.0.2"))
		})

		It("should skip creating a DNSEndpoint when the JSONPath value is missing", func() {
			reconciler := &controller.EndpointReconciler{
				Client: k8sClient,
				Scheme: scheme,
				WatchConfigs: []config.WatchConfig{
					{
						Group:          "",
						Version:        "v1",
						Kind:           testConfigMapKind,
						RecordTemplate: testEndpointRecordTemplate,
						Paths: []config.PathConfig{
							{Path: testEndpointIPPath, Type: "A"},
						},
					},
				},
			}

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-ip-resource",
					Namespace: ns.Name,
				},
				Data: map[string]string{"other": "value"},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			ep := &extdnsv1alpha1.DNSEndpoint{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: ns.Name}, ep)
			Expect(err).To(HaveOccurred())
		})

		It("should support multiple paths producing multiple endpoints", func() {
			reconciler := &controller.EndpointReconciler{
				Client: k8sClient,
				Scheme: scheme,
				WatchConfigs: []config.WatchConfig{
					{
						Group:          "",
						Version:        "v1",
						Kind:           testConfigMapKind,
						RecordTemplate: testEndpointRecordTemplate,
						Paths: []config.PathConfig{
							{Path: testEndpointIPPath, Type: "A"},
							{Path: "$.data.ipv6", Type: "AAAA"},
						},
					},
				},
			}

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dual-stack-resource",
					Namespace: ns.Name,
				},
				Data: map[string]string{
					"ip":   testIPv4,
					"ipv6": "::1",
				},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			ep := &extdnsv1alpha1.DNSEndpoint{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: ns.Name}, ep)).To(Succeed())
			Expect(ep.Spec.Endpoints).To(HaveLen(2))

			recordTypes := make([]string, 0, len(ep.Spec.Endpoints))
			for _, e := range ep.Spec.Endpoints {
				recordTypes = append(recordTypes, e.RecordType)
			}
			Expect(recordTypes).To(ConsistOf("A", "AAAA"))
		})

		It("should skip resources outside the configured watch namespace", func() {
			reconciler := &controller.EndpointReconciler{
				Client: k8sClient,
				Scheme: scheme,
				WatchConfigs: []config.WatchConfig{
					{
						Group:          "",
						Version:        "v1",
						Kind:           testConfigMapKind,
						Namespace:      "some-other-namespace",
						RecordTemplate: testEndpointRecordTemplate,
						Paths: []config.PathConfig{
							{Path: testEndpointIPPath, Type: "A"},
						},
					},
				},
			}

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "filtered-resource",
					Namespace: ns.Name,
				},
				Data: map[string]string{"ip": testIPv4},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			ep := &extdnsv1alpha1.DNSEndpoint{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: ns.Name}, ep)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("SetupWithManager", func() {
		It("should skip registration when no WatchConfigs are configured", func() {
			reconciler := &controller.EndpointReconciler{}
			Expect(reconciler.SetupWithManager(nil)).To(Succeed())
		})
	})
})
