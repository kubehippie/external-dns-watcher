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

package controllers

import (
	"bytes"
	"context"
	"text/template"

	"github.com/PaesslerAG/jsonpath"
	"github.com/kubehippie/external-dns-watcher/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	extdnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	extdns "sigs.k8s.io/external-dns/endpoint"
)

type EndpointReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	WatchConfigs []config.WatchConfig
}

// +kubebuilder:rbac:groups=externaldns.k8s.io,resources=dnsendpoints,verbs=get;list;watch;create;update;patch;delete

func (r *EndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	for _, watch := range r.WatchConfigs {
		if watch.Namespace != "" && watch.Namespace != req.Namespace {
			continue
		}

		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   watch.Group,
			Version: watch.Version,
			Kind:    watch.Kind,
		})

		if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		tmpl, err := template.New(
			"dns",
		).Parse(
			watch.RecordTemplate,
		)

		if err != nil {
			logger.Error(err, "invalid template", "template", watch.RecordTemplate)
			continue
		}

		var (
			buf       bytes.Buffer
			endpoints []*extdns.Endpoint
		)

		_ = tmpl.Execute(&buf, map[string]string{
			"Name":      obj.GetName(),
			"Namespace": obj.GetNamespace(),
		})

		for _, pathCfg := range watch.Paths {
			val, err := jsonpath.Get(
				pathCfg.Path,
				obj.Object,
			)

			if err != nil {
				continue
			}

			switch v := val.(type) {
			case string:
				if v != "" {
					endpoints = append(endpoints, &extdns.Endpoint{
						DNSName:    buf.String(),
						RecordType: pathCfg.Type,
						Targets:    []string{v},
					})
				}
			case []interface{}:
				var (
					targets []string
				)

				for _, x := range v {
					if s, ok := x.(string); ok && s != "" {
						targets = append(targets, s)
					}
				}

				if len(targets) > 0 {
					endpoints = append(endpoints, &extdns.Endpoint{
						DNSName:    buf.String(),
						RecordType: pathCfg.Type,
						Targets:    targets,
					})
				}
			}
		}

		if len(endpoints) == 0 {
			logger.Info("No values extracted, skipping DNSEndpoint", "name", req.NamespacedName)
			continue
		}

		dns := &extdnsv1alpha1.DNSEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
			Spec: extdnsv1alpha1.DNSEndpointSpec{
				Endpoints: endpoints,
			},
		}

		if err := controllerutil.SetControllerReference(obj, dns, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		var (
			existing extdnsv1alpha1.DNSEndpoint
		)

		if err := r.Get(
			ctx,
			client.ObjectKeyFromObject(dns),
			&existing,
		); err != nil {
			if client.IgnoreNotFound(err) == nil {
				if err := r.Create(ctx, dns); err != nil {
					return ctrl.Result{}, err
				}

				logger.Info("Created DNSEndpoint", "name", dns.Name)
			} else {
				return ctrl.Result{}, err
			}
		} else {
			existing.Spec.Endpoints = dns.Spec.Endpoints

			if err := r.Update(ctx, &existing); err != nil {
				return ctrl.Result{}, err
			}

			logger.Info("Updated DNSEndpoint", "name", dns.Name)
		}
	}

	return ctrl.Result{}, nil
}

func (r *EndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr)

	for _, watch := range r.WatchConfigs {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   watch.Group,
			Version: watch.Version,
			Kind:    watch.Kind,
		})

		builder = builder.For(obj)
	}

	builder = builder.Owns(
		&extdnsv1alpha1.DNSEndpoint{},
	)

	return builder.Complete(r)
}
