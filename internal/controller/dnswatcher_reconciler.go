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

package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"text/template"
	"time"

	"github.com/PaesslerAG/jsonpath"
	"github.com/kubehippie/external-dns-watcher/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	extdnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	extdns "sigs.k8s.io/external-dns/endpoint"
)

const (
	// watcherFinalizer is added to DNSWatcher objects to allow cleanup of managed DNSEndpoints.
	watcherFinalizer = "external-dns.webhippie.de/finalizer"

	// labelWatcherName is the label key added to managed DNSEndpoints to identify their owning DNSWatcher.
	labelWatcherName = "external-dns.webhippie.de/watcher"

	// defaultRequeueAfter is the interval for re-checking target resources.
	defaultRequeueAfter = 30 * time.Second
)

// DNSWatcherReconciler reconciles DNSWatcher objects.
type DNSWatcherReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	RequeueAfter time.Duration // defaults to defaultRequeueAfter if zero
}

// +kubebuilder:rbac:groups=external-dns.webhippie.de,resources=dnswatchers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=external-dns.webhippie.de,resources=dnswatchers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=external-dns.webhippie.de,resources=dnswatchers/finalizers,verbs=update
// +kubebuilder:rbac:groups=externaldns.k8s.io,resources=dnsendpoints,verbs=get;list;watch;create;update;patch;delete

func (r *DNSWatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	watcher := &v1alpha1.DNSWatcher{}
	if err := r.Get(ctx, req.NamespacedName, watcher); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !watcher.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, watcher)
	}

	if !controllerutil.ContainsFinalizer(watcher, watcherFinalizer) {
		controllerutil.AddFinalizer(watcher, watcherFinalizer)
		if err := r.Update(ctx, watcher); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// List all target objects for the configured GVK.
	objList := &unstructured.UnstructuredList{}
	objList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   watcher.Spec.Group,
		Version: watcher.Spec.Version,
		Kind:    watcher.Spec.Kind + "List",
	})

	listOpts := []client.ListOption{}
	if watcher.Spec.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(watcher.Spec.Namespace))
	}

	if err := r.List(ctx, objList, listOpts...); err != nil {
		logger.Error(err, "failed to list target resources",
			"group", watcher.Spec.Group,
			"version", watcher.Spec.Version,
			"kind", watcher.Spec.Kind,
		)
		r.setReadyCondition(watcher, metav1.ConditionFalse, "ListFailed", err.Error())
		_ = r.Status().Update(ctx, watcher)
		return ctrl.Result{}, err
	}

	tmpl, err := template.New("dns").Parse(watcher.Spec.RecordTemplate)
	if err != nil {
		r.setReadyCondition(watcher, metav1.ConditionFalse, "InvalidTemplate", err.Error())
		_ = r.Status().Update(ctx, watcher)
		return ctrl.Result{}, err
	}

	// managedEndpoints tracks the (namespace, name) of every DNSEndpoint we create/update this cycle.
	managedEndpoints := make(map[types.NamespacedName]struct{})
	watchedCount := 0

	for i := range objList.Items {
		obj := &objList.Items[i]

		if obj.GetNamespace() == "" {
			logger.Info("skipping cluster-scoped target resource (not supported in v1alpha1)",
				"name", obj.GetName(), "kind", watcher.Spec.Kind)
			continue
		}

		endpoints := r.buildEndpoints(tmpl, obj, watcher.Spec.Paths)
		endpointName := buildEndpointName(watcher.Name, obj.GetName())
		key := types.NamespacedName{Namespace: obj.GetNamespace(), Name: endpointName}

		if len(endpoints) == 0 {
			// Target exists but yields no values — delete stale endpoint if present.
			r.deleteEndpointIfExists(ctx, key)
			continue
		}

		watchedCount++
		managedEndpoints[key] = struct{}{}

		dns := &extdnsv1alpha1.DNSEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      endpointName,
				Namespace: obj.GetNamespace(),
				Labels:    buildLabels(watcher.Name),
			},
			Spec: extdnsv1alpha1.DNSEndpointSpec{
				Endpoints: endpoints,
			},
		}

		if err := controllerutil.SetControllerReference(obj, dns, r.Scheme); err != nil {
			logger.Error(err, "failed to set controller reference", "target", obj.GetName())
		}

		if err := r.createOrUpdateEndpoint(ctx, dns); err != nil {
			logger.Error(err, "failed to create or update DNSEndpoint", "endpoint", endpointName)
			continue
		}
	}

	// Delete any DNSEndpoints labeled for this watcher that were not touched this cycle.
	if err := r.cleanupStaleEndpoints(ctx, watcher.Name, managedEndpoints); err != nil {
		logger.Error(err, "failed to clean up stale DNSEndpoints")
	}

	watcher.Status.ObservedGeneration = watcher.Generation
	watcher.Status.WatchedCount = watchedCount
	r.setReadyCondition(watcher, metav1.ConditionTrue, "ReconcileSucceeded",
		fmt.Sprintf("Watching %d %s resources", watchedCount, watcher.Spec.Kind))

	if err := r.Status().Update(ctx, watcher); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.requeueAfter()}, nil
}

func (r *DNSWatcherReconciler) requeueAfter() time.Duration {
	if r.RequeueAfter > 0 {
		return r.RequeueAfter
	}
	return defaultRequeueAfter
}

func (r *DNSWatcherReconciler) handleDeletion(ctx context.Context, watcher *v1alpha1.DNSWatcher) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if err := r.cleanupStaleEndpoints(ctx, watcher.Name, nil); err != nil {
		logger.Error(err, "failed to clean up DNSEndpoints during deletion")
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(watcher, watcherFinalizer)
	if err := r.Update(ctx, watcher); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DNSWatcherReconciler) buildEndpoints(
	tmpl *template.Template,
	obj *unstructured.Unstructured,
	paths []v1alpha1.PathSpec,
) []*extdns.Endpoint {
	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, map[string]string{
		"Name":      obj.GetName(),
		"Namespace": obj.GetNamespace(),
	})
	dnsName := buf.String()

	var endpoints []*extdns.Endpoint

	for _, pathCfg := range paths {
		val, err := jsonpath.Get(pathCfg.Path, obj.Object)
		if err != nil {
			continue
		}

		switch v := val.(type) {
		case string:
			if v != "" {
				endpoints = append(endpoints, &extdns.Endpoint{
					DNSName:    dnsName,
					RecordType: pathCfg.Type,
					Targets:    []string{v},
				})
			}
		case []interface{}:
			var targets []string
			for _, x := range v {
				if s, ok := x.(string); ok && s != "" {
					targets = append(targets, s)
				}
			}
			if len(targets) > 0 {
				endpoints = append(endpoints, &extdns.Endpoint{
					DNSName:    dnsName,
					RecordType: pathCfg.Type,
					Targets:    targets,
				})
			}
		}
	}

	return endpoints
}

func (r *DNSWatcherReconciler) createOrUpdateEndpoint(ctx context.Context, dns *extdnsv1alpha1.DNSEndpoint) error {
	logger := log.FromContext(ctx)

	var existing extdnsv1alpha1.DNSEndpoint
	key := client.ObjectKeyFromObject(dns)

	if err := r.Get(ctx, key, &existing); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		if err := r.Create(ctx, dns); err != nil {
			return err
		}
		logger.Info("created DNSEndpoint", "name", dns.Name, "namespace", dns.Namespace)
		return nil
	}

	existing.Spec.Endpoints = dns.Spec.Endpoints
	// Preserve labels added by this reconciler.
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	for k, v := range dns.Labels {
		existing.Labels[k] = v
	}

	if err := r.Update(ctx, &existing); err != nil {
		return err
	}
	logger.Info("updated DNSEndpoint", "name", dns.Name, "namespace", dns.Namespace)
	return nil
}

func (r *DNSWatcherReconciler) deleteEndpointIfExists(ctx context.Context, key types.NamespacedName) {
	logger := log.FromContext(ctx)
	var existing extdnsv1alpha1.DNSEndpoint
	if err := r.Get(ctx, key, &existing); err == nil {
		if err := r.Delete(ctx, &existing); err != nil {
			logger.Error(err, "failed to delete stale DNSEndpoint", "name", key.Name, "namespace", key.Namespace)
		} else {
			logger.Info("deleted stale DNSEndpoint (no values extracted)", "name", key.Name, "namespace", key.Namespace)
		}
	}
}

// cleanupStaleEndpoints deletes all DNSEndpoints labeled for the given watcher that are NOT in keepSet.
// Passing nil for keepSet deletes all managed endpoints (used during watcher deletion).
func (r *DNSWatcherReconciler) cleanupStaleEndpoints(
	ctx context.Context,
	watcherName string,
	keepSet map[types.NamespacedName]struct{},
) error {
	logger := log.FromContext(ctx)

	var list extdnsv1alpha1.DNSEndpointList
	if err := r.List(ctx, &list, client.MatchingLabels{labelWatcherName: sanitizeLabelValue(watcherName)}); err != nil {
		return err
	}

	for i := range list.Items {
		ep := &list.Items[i]
		key := types.NamespacedName{Namespace: ep.Namespace, Name: ep.Name}
		if _, keep := keepSet[key]; !keep {
			if err := r.Delete(ctx, ep); err != nil && client.IgnoreNotFound(err) != nil {
				logger.Error(err, "failed to delete stale DNSEndpoint", "name", ep.Name, "namespace", ep.Namespace)
			} else {
				logger.Info("deleted stale DNSEndpoint", "name", ep.Name, "namespace", ep.Namespace)
			}
		}
	}
	return nil
}

func (r *DNSWatcherReconciler) setReadyCondition(watcher *v1alpha1.DNSWatcher, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&watcher.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: watcher.Generation,
	})
}

// SetupWithManager registers the reconciler with the manager.
func (r *DNSWatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DNSWatcher{}).
		Complete(r)
}

// buildEndpointName produces a stable, unique name for a DNSEndpoint managed by the given watcher
// for the given target resource. The result is at most 253 characters.
func buildEndpointName(watcherName, targetName string) string {
	candidate := watcherName + "--" + targetName
	if len(candidate) <= 253 {
		return candidate
	}
	h := sha256.Sum256([]byte(candidate))
	suffix := "-" + hex.EncodeToString(h[:])[:8]
	return candidate[:253-len(suffix)] + suffix
}

// sanitizeLabelValue truncates s to 63 characters (the Kubernetes label value limit).
// If truncation is needed, the last 9 characters are replaced with a hash suffix to avoid collisions.
func sanitizeLabelValue(s string) string {
	if len(s) <= 63 {
		return s
	}
	h := sha256.Sum256([]byte(s))
	suffix := "-" + hex.EncodeToString(h[:])[:8]
	return s[:63-len(suffix)] + suffix
}

// buildLabels returns labels to attach to a managed DNSEndpoint.
func buildLabels(watcherName string) map[string]string {
	return map[string]string{
		labelWatcherName: sanitizeLabelValue(watcherName),
	}
}
