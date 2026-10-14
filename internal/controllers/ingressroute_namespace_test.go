package controllers

import (
	"context"
	"log/slog"
	"testing"

	configv1 "github.com/borchero/switchboard/internal/config/v1"
	"github.com/borchero/switchboard/internal/k8tests"
	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	traefik "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	externaldnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"
)

const namespaceLabelKey = "switchboard.borchero.com/ingress"

func namespaceSelectorConfig() configv1.Config {
	return configv1.Config{
		Selector: configv1.IngressSelector{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{namespaceLabelKey: "enabled"},
			},
		},
		Integrations: configv1.IntegrationConfigs{
			ExternalDNS: &configv1.ExternalDNSIntegrationConfig{
				TargetIPs: []string{"127.0.0.1"},
			},
			CertManager: &configv1.CertManagerIntegrationConfig{
				Template: certmanager.Certificate{
					Spec: certmanager.CertificateSpec{
						IssuerRef: cmmeta.IssuerReference{
							Kind: "ClusterIssuer",
							Name: "my-issuer",
						},
					},
				},
			},
		},
	}
}

func selectorTestIngressRoute(namespace string) traefik.IngressRoute {
	return traefik.IngressRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: namespace,
			UID:       "owner-uid",
		},
		Spec: traefik.IngressRouteSpec{
			Routes: []traefik.Route{{
				Kind:  "Rule",
				Match: "Host(`www.example.com`)",
				Services: []traefik.Service{{
					LoadBalancerSpec: traefik.LoadBalancerSpec{Name: "nginx"},
				}},
			}},
			TLS: &traefik.TLS{
				SecretName: "www-tls-certificate",
			},
		},
	}
}

func reconcileOnce(
	t *testing.T, reconciler IngressRouteReconciler, ingress traefik.IngressRoute,
) {
	_, err := reconciler.Reconcile(context.Background(), controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Name:      ingress.Name,
			Namespace: ingress.Namespace,
		},
	})
	require.Nil(t, err)
}

func TestNewIngressRouteReconcilerNamespaceSelector(t *testing.T) {
	scheme := k8tests.NewScheme()
	ctrlClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// An invalid selector causes construction (and, thus, startup) to fail
	invalid := namespaceSelectorConfig()
	invalid.Selector.NamespaceSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key: "environment", Operator: "Bogus",
		}},
	}
	_, err := NewIngressRouteReconciler(ctrlClient, slog.Default(), invalid)
	require.NotNil(t, err)

	// No selector: cluster-wide operation
	clusterWide, err := NewIngressRouteReconciler(ctrlClient, slog.Default(), configv1.Config{})
	require.Nil(t, err)
	assert.False(t, clusterWide.selector.RestrictsNamespaces())

	// Empty, but present selector: matches all namespaces but installs the namespace gating
	empty := configv1.Config{Selector: configv1.IngressSelector{
		NamespaceSelector: &metav1.LabelSelector{},
	}}
	restricted, err := NewIngressRouteReconciler(ctrlClient, slog.Default(), empty)
	require.Nil(t, err)
	assert.True(t, restricted.selector.RestrictsNamespaces())
}

func TestReconcileNamespaceLookupFailurePreservesResources(t *testing.T) {
	// Setup: the namespace does not exist in the (fake) cache, so looking it up fails.
	scheme := k8tests.NewScheme()
	ingress := selectorTestIngressRoute("missing-namespace")
	certificate := certmanager.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress-tls",
			Namespace: "missing-namespace",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "traefik.io/v1alpha1",
				Kind:       "IngressRoute",
				Name:       "my-ingress",
				UID:        "owner-uid",
				Controller: boolPtr(true),
			}},
		},
	}
	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&ingress, &certificate).
		Build()

	reconciler, err := NewIngressRouteReconciler(ctrlClient, slog.Default(), namespaceSelectorConfig())
	require.Nil(t, err)

	// The reconcile must fail (retriable) and must not delete the existing certificate.
	_, err = reconciler.Reconcile(context.Background(), controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Name:      "my-ingress",
			Namespace: "missing-namespace",
		},
	})
	require.NotNil(t, err)

	var actual certmanager.Certificate
	err = ctrlClient.Get(context.Background(),
		types.NamespacedName{Name: "my-ingress-tls", Namespace: "missing-namespace"}, &actual,
	)
	require.Nil(t, err)
}

func TestReconcileNamespaceSelectorLifecycle(t *testing.T) {
	ctx := context.Background()
	scheme := k8tests.NewScheme()

	namespace := v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-namespace"}}
	ingress := selectorTestIngressRoute(namespace.Name)

	// Foreign resources use the same names as the derived resources but are not owned by the
	// ingress route.
	foreignCertificate := certmanager.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress-tls",
			Namespace: namespace.Name,
			Labels:    map[string]string{"external": "true"},
		},
		Spec: certmanager.CertificateSpec{SecretName: "foreign-secret"},
	}
	foreignEndpoint := externaldnsv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: namespace.Name,
			Labels:    map[string]string{"external": "true"},
		},
		Spec: externaldnsv1alpha1.DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{{
				DNSName:    "foreign.example.com",
				Targets:    []string{"10.0.0.1"},
				RecordType: "A",
			}},
		},
	}

	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&namespace, &ingress, &foreignCertificate, &foreignEndpoint).
		Build()
	reconciler, err := NewIngressRouteReconciler(ctrlClient, slog.Default(), namespaceSelectorConfig())
	require.Nil(t, err)

	certKey := types.NamespacedName{Name: "my-ingress-tls", Namespace: namespace.Name}
	endpointKey := types.NamespacedName{Name: "my-ingress", Namespace: namespace.Name}

	// 1) The namespace does not match initially: foreign resources must be left untouched.
	reconcileOnce(t, reconciler, ingress)

	var actualCertificate certmanager.Certificate
	require.Nil(t, ctrlClient.Get(ctx, certKey, &actualCertificate))
	assert.Equal(t, "foreign-secret", actualCertificate.Spec.SecretName)
	assert.Empty(t, actualCertificate.OwnerReferences)
	assert.Equal(t, "true", actualCertificate.Labels["external"])

	var actualEndpoint externaldnsv1alpha1.DNSEndpoint
	require.Nil(t, ctrlClient.Get(ctx, endpointKey, &actualEndpoint))
	assert.Equal(t, "foreign.example.com", actualEndpoint.Spec.Endpoints[0].DNSName)
	assert.Empty(t, actualEndpoint.OwnerReferences)

	// 2) The label is added: Switchboard takes over (adopts the existing objects) and repairs
	// their specifications.
	var labeledNamespace v1.Namespace
	require.Nil(t, ctrlClient.Get(ctx, types.NamespacedName{Name: namespace.Name}, &labeledNamespace))
	labeledNamespace.Labels = map[string]string{namespaceLabelKey: "enabled"}
	require.Nil(t, ctrlClient.Update(ctx, &labeledNamespace))

	reconcileOnce(t, reconciler, ingress)

	require.Nil(t, ctrlClient.Get(ctx, certKey, &actualCertificate))
	assert.True(t, metav1.IsControlledBy(&actualCertificate, &ingress))
	assert.Equal(t, "www-tls-certificate", actualCertificate.Spec.SecretName)
	assert.ElementsMatch(t, []string{"www.example.com"}, actualCertificate.Spec.DNSNames)
	assert.Equal(t, "switchboard", actualCertificate.Labels["kubernetes.io/managed-by"])

	require.Nil(t, ctrlClient.Get(ctx, endpointKey, &actualEndpoint))
	assert.True(t, metav1.IsControlledBy(&actualEndpoint, &ingress))
	require.Len(t, actualEndpoint.Spec.Endpoints, 1)
	assert.Equal(t, "www.example.com", actualEndpoint.Spec.Endpoints[0].DNSName)
	assert.ElementsMatch(t, []string{"127.0.0.1"}, actualEndpoint.Spec.Endpoints[0].Targets)

	// 3) The label is removed: only the owned resources are deleted. Recreate the foreign
	// resources first to prove they remain untouched.
	var unlabeledNamespace v1.Namespace
	require.Nil(t, ctrlClient.Get(ctx, types.NamespacedName{Name: namespace.Name}, &unlabeledNamespace))
	unlabeledNamespace.Labels = map[string]string{}
	require.Nil(t, ctrlClient.Update(ctx, &unlabeledNamespace))

	reconcileOnce(t, reconciler, ingress)

	err = ctrlClient.Get(ctx, certKey, &certmanager.Certificate{})
	assert.True(t, apierrors.IsNotFound(err))
	err = ctrlClient.Get(ctx, endpointKey, &externaldnsv1alpha1.DNSEndpoint{})
	assert.True(t, apierrors.IsNotFound(err))

	// 4) The label is restored: the derived resources are rebuilt.
	var relabeledNamespace v1.Namespace
	require.Nil(t, ctrlClient.Get(ctx, types.NamespacedName{Name: namespace.Name}, &relabeledNamespace))
	relabeledNamespace.Labels = map[string]string{namespaceLabelKey: "enabled"}
	require.Nil(t, ctrlClient.Update(ctx, &relabeledNamespace))

	reconcileOnce(t, reconciler, ingress)

	require.Nil(t, ctrlClient.Get(ctx, certKey, &actualCertificate))
	assert.True(t, metav1.IsControlledBy(&actualCertificate, &ingress))
	assert.Equal(t, "www-tls-certificate", actualCertificate.Spec.SecretName)
	require.Nil(t, ctrlClient.Get(ctx, endpointKey, &actualEndpoint))
	assert.True(t, metav1.IsControlledBy(&actualEndpoint, &ingress))
	require.Len(t, actualEndpoint.Spec.Endpoints, 1)
	assert.ElementsMatch(t, []string{"127.0.0.1"}, actualEndpoint.Spec.Endpoints[0].Targets)
}

func TestReconcileOutOfScopeKeepsForeignResources(t *testing.T) {
	ctx := context.Background()
	scheme := k8tests.NewScheme()

	// The namespace is explicitly labeled with a non-matching value.
	namespace := v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "my-namespace",
		Labels: map[string]string{namespaceLabelKey: "disabled"},
	}}
	ingress := selectorTestIngressRoute(namespace.Name)
	foreignCertificate := certmanager.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress-tls",
			Namespace: namespace.Name,
			Labels:    map[string]string{"external": "true"},
		},
		Spec: certmanager.CertificateSpec{SecretName: "foreign-secret"},
	}
	foreignEndpoint := externaldnsv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: namespace.Name,
			Labels:    map[string]string{"external": "true"},
		},
	}

	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&namespace, &ingress, &foreignCertificate, &foreignEndpoint).
		Build()
	reconciler, err := NewIngressRouteReconciler(ctrlClient, slog.Default(), namespaceSelectorConfig())
	require.Nil(t, err)

	reconcileOnce(t, reconciler, ingress)

	var actualCertificate certmanager.Certificate
	require.Nil(t, ctrlClient.Get(ctx,
		types.NamespacedName{Name: "my-ingress-tls", Namespace: namespace.Name}, &actualCertificate,
	))
	assert.Equal(t, "foreign-secret", actualCertificate.Spec.SecretName)
	assert.Empty(t, actualCertificate.OwnerReferences)

	var actualEndpoint externaldnsv1alpha1.DNSEndpoint
	require.Nil(t, ctrlClient.Get(ctx,
		types.NamespacedName{Name: "my-ingress", Namespace: namespace.Name}, &actualEndpoint,
	))
	assert.Empty(t, actualEndpoint.OwnerReferences)
	assert.Equal(t, "true", actualEndpoint.Labels["external"])
}

func boolPtr(value bool) *bool {
	return &value
}
