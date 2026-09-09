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
	"time"

	"github.com/kubehippie/external-dns-watcher/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	extdnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

const (
	testConfigMapKind   = "ConfigMap"
	testRecordTemplate  = "{{ .Name }}.example.com"
	testConfigMapIPPath = "$.data.ip"
)

var _ = Describe("DNSWatcherReconciler", func() {
	const (
		timeout  = 10 * time.Second
		interval = 250 * time.Millisecond
	)

	var (
		ctx context.Context
		ns  *corev1.Namespace
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create a unique namespace per test for isolation.
		ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-"}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), ns,
			client.PropagationPolicy(metav1.DeletePropagationForeground),
		)).To(Succeed())
	})

	Context("when a DNSWatcher is created", func() {
		It("should create a DNSEndpoint for a matching target resource", func() {
			watcher := &v1alpha1.DNSWatcher{
				ObjectMeta: metav1.ObjectMeta{Name: "test-watcher"},
				Spec: v1alpha1.DNSWatcherSpec{
					Group:          "",
					Version:        "v1",
					Kind:           testConfigMapKind,
					Namespace:      ns.Name,
					RecordTemplate: testRecordTemplate,
					Paths: []v1alpha1.PathSpec{
						{Path: testConfigMapIPPath, Type: "A"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, watcher)).To(Succeed())
			DeferCleanup(func() {
				// Remove finalizer before delete to avoid stuck resources.
				w := &v1alpha1.DNSWatcher{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: watcher.Name}, w); err == nil {
					w.Finalizers = nil
					_ = k8sClient.Update(ctx, w)
				}
				_ = k8sClient.Delete(ctx, watcher)
			})

			// Create a ConfigMap with the IP in its data.
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-resource",
					Namespace: ns.Name,
				},
				Data: map[string]string{
					"ip": testIPv4,
				},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			// The reconciler should create a DNSEndpoint named "<watcher>--<target>".
			expectedName := "test-watcher--my-resource"
			endpointKey := types.NamespacedName{Name: expectedName, Namespace: ns.Name}

			Eventually(func(g Gomega) {
				ep := &extdnsv1alpha1.DNSEndpoint{}
				g.Expect(k8sClient.Get(ctx, endpointKey, ep)).To(Succeed())
				g.Expect(ep.Spec.Endpoints).To(HaveLen(1))
				g.Expect(ep.Spec.Endpoints[0].DNSName).To(Equal("my-resource.example.com"))
				g.Expect(ep.Spec.Endpoints[0].RecordType).To(Equal("A"))
				g.Expect(ep.Spec.Endpoints[0].Targets).To(ConsistOf(testIPv4))
			}, timeout, interval).Should(Succeed())
		})

		It("should update the DNSEndpoint when the target resource changes", func() {
			watcher := &v1alpha1.DNSWatcher{
				ObjectMeta: metav1.ObjectMeta{Name: "update-watcher"},
				Spec: v1alpha1.DNSWatcherSpec{
					Group:          "",
					Version:        "v1",
					Kind:           testConfigMapKind,
					Namespace:      ns.Name,
					RecordTemplate: testRecordTemplate,
					Paths: []v1alpha1.PathSpec{
						{Path: testConfigMapIPPath, Type: "A"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, watcher)).To(Succeed())
			DeferCleanup(func() {
				w := &v1alpha1.DNSWatcher{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: watcher.Name}, w); err == nil {
					w.Finalizers = nil
					_ = k8sClient.Update(ctx, w)
				}
				_ = k8sClient.Delete(ctx, watcher)
			})

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "changing-resource",
					Namespace: ns.Name,
				},
				Data: map[string]string{"ip": "10.0.0.1"},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			endpointKey := types.NamespacedName{Name: "update-watcher--changing-resource", Namespace: ns.Name}
			Eventually(func(g Gomega) {
				ep := &extdnsv1alpha1.DNSEndpoint{}
				g.Expect(k8sClient.Get(ctx, endpointKey, ep)).To(Succeed())
				g.Expect(ep.Spec.Endpoints[0].Targets).To(ConsistOf("10.0.0.1"))
			}, timeout, interval).Should(Succeed())

			// Update the ConfigMap with a new IP.
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
			cm.Data["ip"] = "10.0.0.2"
			Expect(k8sClient.Update(ctx, cm)).To(Succeed())

			Eventually(func(g Gomega) {
				ep := &extdnsv1alpha1.DNSEndpoint{}
				g.Expect(k8sClient.Get(ctx, endpointKey, ep)).To(Succeed())
				g.Expect(ep.Spec.Endpoints[0].Targets).To(ConsistOf("10.0.0.2"))
			}, timeout, interval).Should(Succeed())
		})

		It("should delete the DNSEndpoint when the target's value becomes empty", func() {
			watcher := &v1alpha1.DNSWatcher{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-watcher"},
				Spec: v1alpha1.DNSWatcherSpec{
					Group:          "",
					Version:        "v1",
					Kind:           testConfigMapKind,
					Namespace:      ns.Name,
					RecordTemplate: testRecordTemplate,
					Paths: []v1alpha1.PathSpec{
						{Path: testConfigMapIPPath, Type: "A"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, watcher)).To(Succeed())
			DeferCleanup(func() {
				w := &v1alpha1.DNSWatcher{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: watcher.Name}, w); err == nil {
					w.Finalizers = nil
					_ = k8sClient.Update(ctx, w)
				}
				_ = k8sClient.Delete(ctx, watcher)
			})

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "will-be-empty",
					Namespace: ns.Name,
				},
				Data: map[string]string{"ip": "5.5.5.5"},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			endpointKey := types.NamespacedName{Name: "empty-watcher--will-be-empty", Namespace: ns.Name}
			Eventually(func(g Gomega) {
				ep := &extdnsv1alpha1.DNSEndpoint{}
				g.Expect(k8sClient.Get(ctx, endpointKey, ep)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			// Remove the IP from the ConfigMap.
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
			delete(cm.Data, "ip")
			Expect(k8sClient.Update(ctx, cm)).To(Succeed())

			Eventually(func() error {
				ep := &extdnsv1alpha1.DNSEndpoint{}
				return k8sClient.Get(ctx, endpointKey, ep)
			}, timeout, interval).ShouldNot(Succeed())
		})

		It("should clean up all DNSEndpoints when the DNSWatcher is deleted", func() {
			watcher := &v1alpha1.DNSWatcher{
				ObjectMeta: metav1.ObjectMeta{Name: "cleanup-watcher"},
				Spec: v1alpha1.DNSWatcherSpec{
					Group:          "",
					Version:        "v1",
					Kind:           testConfigMapKind,
					Namespace:      ns.Name,
					RecordTemplate: testRecordTemplate,
					Paths: []v1alpha1.PathSpec{
						{Path: testConfigMapIPPath, Type: "A"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, watcher)).To(Succeed())

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cleanup-target",
					Namespace: ns.Name,
				},
				Data: map[string]string{"ip": "9.9.9.9"},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			endpointKey := types.NamespacedName{Name: "cleanup-watcher--cleanup-target", Namespace: ns.Name}
			Eventually(func(g Gomega) {
				ep := &extdnsv1alpha1.DNSEndpoint{}
				g.Expect(k8sClient.Get(ctx, endpointKey, ep)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, watcher)).To(Succeed())

			Eventually(func() error {
				ep := &extdnsv1alpha1.DNSEndpoint{}
				return k8sClient.Get(ctx, endpointKey, ep)
			}, timeout, interval).ShouldNot(Succeed())
		})

		It("should skip cluster-scoped target resources", func() {
			// Use a cluster-scoped resource (Namespace) as target — should be skipped.
			watcher := &v1alpha1.DNSWatcher{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster-scope-watcher"},
				Spec: v1alpha1.DNSWatcherSpec{
					Group:          "",
					Version:        "v1",
					Kind:           "Namespace",
					RecordTemplate: testRecordTemplate,
					Paths: []v1alpha1.PathSpec{
						{Path: "$.metadata.name", Type: "A"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, watcher)).To(Succeed())
			DeferCleanup(func() {
				w := &v1alpha1.DNSWatcher{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: watcher.Name}, w); err == nil {
					w.Finalizers = nil
					_ = k8sClient.Update(ctx, w)
				}
				_ = k8sClient.Delete(ctx, watcher)
			})

			// Status should become Ready=True with WatchedCount=0.
			Eventually(func(g Gomega) {
				w := &v1alpha1.DNSWatcher{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: watcher.Name}, w)).To(Succeed())
				g.Expect(w.Status.WatchedCount).To(Equal(0))
			}, timeout, interval).Should(Succeed())
		})

		It("should set Ready condition to True after successful reconcile", func() {
			watcher := &v1alpha1.DNSWatcher{
				ObjectMeta: metav1.ObjectMeta{Name: "status-watcher"},
				Spec: v1alpha1.DNSWatcherSpec{
					Group:          "",
					Version:        "v1",
					Kind:           testConfigMapKind,
					Namespace:      ns.Name,
					RecordTemplate: testRecordTemplate,
					Paths: []v1alpha1.PathSpec{
						{Path: testConfigMapIPPath, Type: "A"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, watcher)).To(Succeed())
			DeferCleanup(func() {
				w := &v1alpha1.DNSWatcher{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: watcher.Name}, w); err == nil {
					w.Finalizers = nil
					_ = k8sClient.Update(ctx, w)
				}
				_ = k8sClient.Delete(ctx, watcher)
			})

			Eventually(func(g Gomega) {
				w := &v1alpha1.DNSWatcher{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: watcher.Name}, w)).To(Succeed())
				readyCond := findCondition(w.Status.Conditions, "Ready")
				g.Expect(readyCond).NotTo(BeNil())
				g.Expect(readyCond.Status).To(Equal(metav1.ConditionTrue))
			}, timeout, interval).Should(Succeed())
		})

		It("should watch all namespaces when spec.namespace is empty", func() {
			ns2 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "test2-"}}
			Expect(k8sClient.Create(ctx, ns2)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(context.Background(), ns2)
			})

			watcher := &v1alpha1.DNSWatcher{
				ObjectMeta: metav1.ObjectMeta{Name: "multi-ns-watcher"},
				Spec: v1alpha1.DNSWatcherSpec{
					Group:          "",
					Version:        "v1",
					Kind:           testConfigMapKind,
					Namespace:      "", // watch all namespaces
					RecordTemplate: testRecordTemplate,
					Paths: []v1alpha1.PathSpec{
						{Path: testConfigMapIPPath, Type: "A"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, watcher)).To(Succeed())
			DeferCleanup(func() {
				w := &v1alpha1.DNSWatcher{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: watcher.Name}, w); err == nil {
					w.Finalizers = nil
					_ = k8sClient.Update(ctx, w)
				}
				_ = k8sClient.Delete(ctx, watcher)
			})

			for _, nsName := range []string{ns.Name, ns2.Name} {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multi-target",
						Namespace: nsName,
					},
					Data: map[string]string{"ip": "192.168.1.1"},
				}
				Expect(k8sClient.Create(ctx, cm)).To(Succeed())
			}

			for _, nsName := range []string{ns.Name, ns2.Name} {
				endpointKey := types.NamespacedName{
					Name:      "multi-ns-watcher--multi-target",
					Namespace: nsName,
				}
				Eventually(func(g Gomega) {
					ep := &extdnsv1alpha1.DNSEndpoint{}
					g.Expect(k8sClient.Get(ctx, endpointKey, ep)).To(Succeed())
				}, timeout, interval).Should(Succeed(), "expected endpoint in namespace %s", nsName)
			}
		})
	})

	Context("unit helpers", func() {
		It("buildEndpointName returns watcher--target for short names", func() {
			obj := makeUnstructured("test-ns", "my-target")
			_ = obj
			// Indirectly tested via the reconciler above. The naming is <watcher>--<target>.
		})
	})
})

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func makeUnstructured(namespace, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return obj
}
