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

package v1alpha1

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	apiv1alpha1 "github.com/kubehippie/external-dns-watcher/api/v1alpha1"
)

var dnswatcherlog = logf.Log.WithName("dnswatcher-resource")

var validDNSTypes = map[string]struct{}{
	"A":     {},
	"AAAA":  {},
	"CNAME": {},
	"TXT":   {},
	"MX":    {},
	"NS":    {},
	"SRV":   {},
	"PTR":   {},
	"SOA":   {},
	"CAA":   {},
	"NAPTR": {},
}

// SetupDNSWatcherWebhookWithManager registers the mutating and validating webhooks for DNSWatcher.
func SetupDNSWatcherWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &apiv1alpha1.DNSWatcher{}).
		WithDefaulter(&DNSWatcherCustomDefaulter{}).
		WithValidator(&DNSWatcherCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-external-dns-webhippie-de-v1alpha1-dnswatcher,mutating=true,failurePolicy=fail,sideEffects=None,groups=external-dns.webhippie.de,resources=dnswatchers,verbs=create;update,versions=v1alpha1,name=mdnswatcher.kb.io,admissionReviewVersions=v1

// DNSWatcherCustomDefaulter sets defaults for DNSWatcher objects.
type DNSWatcherCustomDefaulter struct{}

// Default normalises DNS record types to uppercase before the object is stored.
func (d *DNSWatcherCustomDefaulter) Default(_ context.Context, obj *apiv1alpha1.DNSWatcher) error {
	dnswatcherlog.Info("defaulting", "name", obj.Name)
	for i := range obj.Spec.Paths {
		obj.Spec.Paths[i].Type = strings.ToUpper(obj.Spec.Paths[i].Type)
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-external-dns-webhippie-de-v1alpha1-dnswatcher,mutating=false,failurePolicy=fail,sideEffects=None,groups=external-dns.webhippie.de,resources=dnswatchers,verbs=create;update,versions=v1alpha1,name=vdnswatcher.kb.io,admissionReviewVersions=v1

// DNSWatcherCustomValidator validates DNSWatcher objects on create and update.
type DNSWatcherCustomValidator struct{}

// ValidateCreate validates the object on creation.
func (v *DNSWatcherCustomValidator) ValidateCreate(_ context.Context, obj *apiv1alpha1.DNSWatcher) (admission.Warnings, error) {
	dnswatcherlog.Info("validate create", "name", obj.Name)
	return nil, validateSpec(obj)
}

// ValidateUpdate validates the object on update.
func (v *DNSWatcherCustomValidator) ValidateUpdate(_ context.Context, _, obj *apiv1alpha1.DNSWatcher) (admission.Warnings, error) {
	dnswatcherlog.Info("validate update", "name", obj.Name)
	return nil, validateSpec(obj)
}

// ValidateDelete validates the object on deletion.
func (v *DNSWatcherCustomValidator) ValidateDelete(_ context.Context, _ *apiv1alpha1.DNSWatcher) (admission.Warnings, error) {
	return nil, nil
}

func validateSpec(r *apiv1alpha1.DNSWatcher) error {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	if r.Spec.Version == "" {
		errs = append(errs, field.Required(specPath.Child("version"), "version must not be empty"))
	}
	if r.Spec.Kind == "" {
		errs = append(errs, field.Required(specPath.Child("kind"), "kind must not be empty"))
	}
	if r.Spec.RecordTemplate == "" {
		errs = append(errs, field.Required(specPath.Child("recordTemplate"), "recordTemplate must not be empty"))
	} else {
		if err := validateTemplate(r.Spec.RecordTemplate); err != nil {
			errs = append(errs, field.Invalid(specPath.Child("recordTemplate"), r.Spec.RecordTemplate, err.Error()))
		}
	}
	if len(r.Spec.Paths) == 0 {
		errs = append(errs, field.Required(specPath.Child("paths"), "at least one path must be specified"))
	}
	for i, p := range r.Spec.Paths {
		pathField := specPath.Child("paths").Index(i)
		if p.Path == "" {
			errs = append(errs, field.Required(pathField.Child("path"), "path must not be empty"))
		} else if !strings.HasPrefix(p.Path, "$") {
			errs = append(errs, field.Invalid(pathField.Child("path"), p.Path, "path must be a valid JSONPath expression starting with '$'"))
		}
		if p.Type == "" {
			errs = append(errs, field.Required(pathField.Child("type"), "type must not be empty"))
		} else if _, ok := validDNSTypes[strings.ToUpper(p.Type)]; !ok {
			errs = append(errs, field.NotSupported(pathField.Child("type"), p.Type, validDNSTypeList()))
		}
	}

	if len(errs) > 0 {
		return apierrors.NewInvalid(
			r.GroupVersionKind().GroupKind(),
			r.Name,
			errs,
		)
	}
	return nil
}

func validateTemplate(tmplStr string) error {
	tmpl, err := template.New("validate").Option("missingkey=error").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("invalid Go template: %w", err)
	}
	// Execute with dummy data to catch unknown key references.
	dummy := map[string]string{"Name": "test", "Namespace": "test"}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, dummy); err != nil {
		return fmt.Errorf("template execution failed (check .Name and .Namespace are the only variables): %w", err)
	}
	return nil
}

func validDNSTypeList() []string {
	types := make([]string, 0, len(validDNSTypes))
	for t := range validDNSTypes {
		types = append(types, t)
	}
	return types
}
