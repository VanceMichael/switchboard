package controllers

import (
	"context"
	"log/slog"
	"testing"

	configv1 "github.com/borchero/switchboard/internal/config/v1"
	"github.com/borchero/switchboard/internal/k8tests"
	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	traefik "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	externaldnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

// reconcileWithClient runs a single reconciliation pass against the provided (fake) client.
func reconcileWithClient(
	ctx context.Context, t *testing.T,
	ctrlClient client.Client, ingress traefik.IngressRoute, config configv1.Config,
) {
	reconciler, err := NewIngressRouteReconciler(ctrlClient, slog.Default(), config)
	require.Nil(t, err)
	_, err = reconciler.Reconcile(ctx, controllerruntime.Request{
		NamespacedName: types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace},
	})
	require.Nil(t, err)
}

func boolPtr(value bool) *bool {
	return &value
}

func headlessService(name, namespace string) v1.Service {
	return v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.ServiceSpec{
			ClusterIP:  v1.ClusterIPNone,
			ClusterIPs: []string{v1.ClusterIPNone},
			Ports:      []v1.ServicePort{{Port: 80, Name: "http"}},
		},
	}
}

func endpointSlice(
	name, namespace, serviceName string, addressType discoveryv1.AddressType,
	ready bool, addresses ...string,
) discoveryv1.EndpointSlice {
	return discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{discoveryv1.LabelServiceName: serviceName},
		},
		AddressType: addressType,
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  addresses,
			Conditions: discoveryv1.EndpointConditions{Ready: &ready},
		}},
	}
}

func headlessIngressRoute(name, namespace string) traefik.IngressRoute {
	return traefik.IngressRoute{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: traefik.IngressRouteSpec{
			Routes: []traefik.Route{{
				Kind:  "Rule",
				Match: "Host(`www.example.com`)",
				Services: []traefik.Service{{
					LoadBalancerSpec: traefik.LoadBalancerSpec{Name: "nginx"},
				}},
			}},
			TLS: &traefik.TLS{SecretName: "www-tls-certificate"},
		},
	}
}

func TestReconcileHeadlessServiceEndpointSlices(t *testing.T) {
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	const namespace = "ns"

	service := headlessService("traefik", namespace)
	ingress := headlessIngressRoute("my-ingress", namespace)

	ipv4Slice := endpointSlice("traefik-v4", namespace, service.Name,
		discoveryv1.AddressTypeIPv4, true, "10.0.1.1", "10.0.1.2")
	ipv6Slice := endpointSlice("traefik-v6", namespace, service.Name,
		discoveryv1.AddressTypeIPv6, true, "2001:db8::1")
	// Slice of an unrelated service must never contribute targets.
	otherSlice := endpointSlice("other-v4", namespace, "other-service",
		discoveryv1.AddressTypeIPv4, true, "10.0.9.9")

	ctrlClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&service, &ingress, &ipv4Slice, &ipv6Slice, &otherSlice,
	).Build()
	config := createConfig(&service)

	reconcileWithClient(ctx, t, ctrlClient, ingress, config)

	// The DNS endpoint must publish the ready IPv4/IPv6 addresses of the target's slices and
	// the certificate integration must still work unchanged.
	dnsEndpoint := getDNSEndpoint(t, ctx, ctrlClient, ingress)
	targets := dnsTargetsByRecord(dnsEndpoint)
	assert.ElementsMatch(t, []string{"10.0.1.1", "10.0.1.2"}, targets["A"])
	assert.ElementsMatch(t, []string{"2001:db8::1"}, targets["AAAA"])
	assert.NotContains(t, targets, "CNAME")
	for _, ep := range dnsEndpoint.Spec.Endpoints {
		for _, address := range ep.Targets {
			assert.NotEqual(t, v1.ClusterIPNone, address)
		}
	}
	certificate := getCertificate(t, ctx, ctrlClient, namespace, "my-ingress-tls")
	assert.ElementsMatch(t, []string{"www.example.com"}, certificate.Spec.DNSNames)

	// Pods become unready: the stale DNS endpoint is withdrawn, the certificate is untouched.
	ipv4Slice.Endpoints[0].Conditions.Ready = boolPtr(false)
	ipv6Slice.Endpoints[0].Conditions.Ready = boolPtr(false)
	require.Nil(t, ctrlClient.Update(ctx, &ipv4Slice))
	require.Nil(t, ctrlClient.Update(ctx, &ipv6Slice))
	reconcileWithClient(ctx, t, ctrlClient, ingress, config)
	assertDNSEndpointNotFound(t, ctx, ctrlClient, ingress)
	getCertificate(t, ctx, ctrlClient, namespace, "my-ingress-tls")

	// Ready addresses reappear: the DNS endpoint is restored automatically.
	ipv4Slice.Endpoints[0].Conditions.Ready = boolPtr(true)
	ipv6Slice.Endpoints[0].Conditions.Ready = boolPtr(true)
	require.Nil(t, ctrlClient.Update(ctx, &ipv4Slice))
	require.Nil(t, ctrlClient.Update(ctx, &ipv6Slice))
	reconcileWithClient(ctx, t, ctrlClient, ingress, config)
	dnsEndpoint = getDNSEndpoint(t, ctx, ctrlClient, ingress)
	targets = dnsTargetsByRecord(dnsEndpoint)
	assert.ElementsMatch(t, []string{"10.0.1.1", "10.0.1.2"}, targets["A"])
	assert.ElementsMatch(t, []string{"2001:db8::1"}, targets["AAAA"])

	// Slice deletion withdraws the affected addresses.
	require.Nil(t, ctrlClient.Delete(ctx, &ipv6Slice))
	reconcileWithClient(ctx, t, ctrlClient, ingress, config)
	dnsEndpoint = getDNSEndpoint(t, ctx, ctrlClient, ingress)
	targets = dnsTargetsByRecord(dnsEndpoint)
	assert.ElementsMatch(t, []string{"10.0.1.1", "10.0.1.2"}, targets["A"])
	assert.NotContains(t, targets, "AAAA")

	// Switching the service from headless to a regular (dual-stack) address source publishes
	// the cluster IPs instead of endpoint slice addresses.
	var updated v1.Service
	require.Nil(t, ctrlClient.Get(ctx,
		types.NamespacedName{Name: service.Name, Namespace: namespace}, &updated))
	updated.Spec.ClusterIP = "10.96.0.1"
	updated.Spec.ClusterIPs = []string{"10.96.0.1", "2001:db8::96"}
	require.Nil(t, ctrlClient.Update(ctx, &updated))
	reconcileWithClient(ctx, t, ctrlClient, ingress, config)
	dnsEndpoint = getDNSEndpoint(t, ctx, ctrlClient, ingress)
	targets = dnsTargetsByRecord(dnsEndpoint)
	assert.ElementsMatch(t, []string{"10.96.0.1"}, targets["A"])
	assert.ElementsMatch(t, []string{"2001:db8::96"}, targets["AAAA"])
}

func TestReconcileDualStackNormalService(t *testing.T) {
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	const namespace = "ns"

	service := v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "traefik", Namespace: namespace},
		Spec: v1.ServiceSpec{
			ClusterIP:  "10.96.0.10",
			ClusterIPs: []string{"10.96.0.10", "2001:db8::10"},
			Ports:      []v1.ServicePort{{Port: 80, Name: "http"}},
		},
	}
	ingress := headlessIngressRoute("my-ingress", namespace)
	// Even if slices exist, a non-headless service keeps sourcing targets from its cluster IP.
	slice := endpointSlice("traefik-v4", namespace, service.Name,
		discoveryv1.AddressTypeIPv4, true, "10.0.1.1")

	ctrlClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&service, &ingress, &slice,
	).Build()
	config := createConfig(&service)
	reconcileWithClient(ctx, t, ctrlClient, ingress, config)

	dnsEndpoint := getDNSEndpoint(t, ctx, ctrlClient, ingress)
	targets := dnsTargetsByRecord(dnsEndpoint)
	assert.ElementsMatch(t, []string{"10.96.0.10"}, targets["A"])
	assert.ElementsMatch(t, []string{"2001:db8::10"}, targets["AAAA"])
}

//-------------------------------------------------------------------------------------------------
// HELPERS
//-------------------------------------------------------------------------------------------------

func getDNSEndpoint(
	t *testing.T, ctx context.Context, ctrlClient client.Client, ingress traefik.IngressRoute,
) externaldnsv1alpha1.DNSEndpoint {
	var dnsEndpoint externaldnsv1alpha1.DNSEndpoint
	err := ctrlClient.Get(ctx,
		types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, &dnsEndpoint)
	require.Nil(t, err)
	return dnsEndpoint
}

func assertDNSEndpointNotFound(
	t *testing.T, ctx context.Context, ctrlClient client.Client, ingress traefik.IngressRoute,
) {
	var dnsEndpoint externaldnsv1alpha1.DNSEndpoint
	err := ctrlClient.Get(ctx,
		types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, &dnsEndpoint)
	assert.True(t, apierrors.IsNotFound(err))
}

func getCertificate(
	t *testing.T, ctx context.Context, ctrlClient client.Client, namespace, name string,
) certmanager.Certificate {
	var certificate certmanager.Certificate
	err := ctrlClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &certificate)
	require.Nil(t, err)
	return certificate
}

func dnsTargetsByRecord(endpoint externaldnsv1alpha1.DNSEndpoint) map[string][]string {
	result := make(map[string][]string)
	for _, record := range endpoint.Spec.Endpoints {
		result[record.RecordType] = append(result[record.RecordType], record.Targets...)
	}
	return result
}
