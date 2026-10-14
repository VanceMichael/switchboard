package integrations

import (
	"context"
	"testing"

	"github.com/borchero/switchboard/internal/k8tests"
	"github.com/borchero/switchboard/internal/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	externaldnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"
)

func boolPtr(value bool) *bool {
	return &value
}

func TestExternalDNSWatchedObjects(t *testing.T) {
	integration := NewExternalDNS(nil, switchboard.NewServiceTarget("my-name", "my-namespace"), nil)
	objects := integration.WatchedObjects()
	require.Len(t, objects, 2)

	// The target service itself is watched by exact name/namespace.
	var service *v1.Service
	var slice *discoveryv1.EndpointSlice
	for _, obj := range objects {
		switch typed := obj.(type) {
		case *v1.Service:
			service = typed
		case *discoveryv1.EndpointSlice:
			slice = typed
		}
	}
	require.NotNil(t, service)
	assert.Equal(t, "my-name", service.GetName())
	assert.Equal(t, "my-namespace", service.GetNamespace())

	// The endpoint slices of the target service are watched in the same namespace via the
	// `kubernetes.io/service-name` label (slice names are generated).
	require.NotNil(t, slice)
	assert.Equal(t, "my-namespace", slice.GetNamespace())
	assert.Empty(t, slice.GetName())
	assert.Equal(t, "my-name", slice.GetLabels()[discoveryv1.LabelServiceName])

	// Static targets do not watch any resources.
	static := NewExternalDNS(nil, switchboard.NewStaticTarget("127.0.0.1"), nil)
	assert.Nil(t, static.WatchedObjects())
}

func TestExternalDNSHeadlessLifecycle(t *testing.T) {
	// Uses a fake client to verify that DNS endpoints are withdrawn while a headless service has
	// no ready endpoints and restored once addresses become available again.
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	const namespace = "ns"
	const serviceName = "headless"

	headless := v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: namespace},
		Spec: v1.ServiceSpec{
			ClusterIP:  v1.ClusterIPNone,
			ClusterIPs: []string{v1.ClusterIPNone},
			Ports:      []v1.ServicePort{{Port: 80, Name: "http"}},
		},
	}
	owner := v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: namespace},
	}
	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).WithObjects(&headless, &owner).Build()
	integration := NewExternalDNS(ctrlClient,
		switchboard.NewServiceTarget(serviceName, namespace), nil)

	info := IngressInfo{Hosts: []string{"example.com"}}

	// No ready endpoints: the stale endpoint must be withdrawn (and no error occurs).
	require.Nil(t, integration.UpdateResource(ctx, &owner, info))
	var endpoints externaldnsv1alpha1.DNSEndpointList
	require.Nil(t, ctrlClient.List(ctx, &endpoints, client.InNamespace(namespace)))
	assert.Len(t, endpoints.Items, 0)

	// Once a ready slice address exists, the endpoint is published.
	slice := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "headless-v4",
			Namespace: namespace,
			Labels:    map[string]string{discoveryv1.LabelServiceName: serviceName},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{"10.0.1.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)},
		}},
	}
	require.Nil(t, ctrlClient.Create(ctx, &slice))
	require.Nil(t, integration.UpdateResource(ctx, &owner, info))
	require.Nil(t, ctrlClient.List(ctx, &endpoints, client.InNamespace(namespace)))
	require.Len(t, endpoints.Items, 1)
	assert.ElementsMatch(t, []string{"10.0.1.1"}, endpoints.Items[0].Spec.Endpoints[0].Targets)

	// Pod going unready withdraws the endpoint again.
	slice.Endpoints[0].Conditions.Ready = boolPtr(false)
	require.Nil(t, ctrlClient.Update(ctx, &slice))
	require.Nil(t, integration.UpdateResource(ctx, &owner, info))
	endpoints = externaldnsv1alpha1.DNSEndpointList{}
	require.Nil(t, ctrlClient.List(ctx, &endpoints, client.InNamespace(namespace)))
	assert.Len(t, endpoints.Items, 0)
}

func TestExternalDNSUpdateResource(t *testing.T) {
	// Setup
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	client := k8tests.NewClient(t, scheme)
	namespace, shutdown := k8tests.NewNamespace(ctx, t, client)
	defer shutdown()

	// Create a dummy service as owner and target
	owner := k8tests.DummyService("my-service", namespace, 80)
	err := client.Create(ctx, &owner)
	require.Nil(t, err)
	integration := NewExternalDNS(client, switchboard.NewServiceTarget(owner.Name, namespace), nil)

	// No resource should be created if no hosts are provided
	var info IngressInfo

	err = integration.UpdateResource(ctx, &owner, info)
	require.Nil(t, err)
	assert.Len(t, getDNSEndpoints(ctx, t, client, namespace), 0)

	// A resource with the name of the service should be created for at least one host
	info.Hosts = []string{"example.com"}
	err = integration.UpdateResource(ctx, &owner, info)
	require.Nil(t, err)
	endpoints := getDNSEndpoints(ctx, t, client, namespace)
	assert.Len(t, endpoints, 1)
	assert.Contains(t, endpoints, owner.Name)
	assert.ElementsMatch(t, endpoints[owner.Name], info.Hosts)

	// When the hosts are changed, more endpoints should be added
	info.Hosts = []string{"example.com", "www.example.com"}
	err = integration.UpdateResource(ctx, &owner, info)
	require.Nil(t, err)
	endpoints = getDNSEndpoints(ctx, t, client, namespace)
	assert.Len(t, endpoints, 1)
	assert.Contains(t, endpoints, owner.Name)
	assert.ElementsMatch(t, endpoints[owner.Name], info.Hosts)

	// When no hosts are set, the endpoints should be removed
	info.Hosts = nil
	err = integration.UpdateResource(ctx, &owner, info)
	require.Nil(t, err)
	assert.Len(t, getDNSEndpoints(ctx, t, client, namespace), 0)
}

func TestExternalDNSEndpoints(t *testing.T) {
	integration := externalDNS{ttl: 250}
	hosts := []string{"example.com", "www.example.com"}

	endpoints := integration.endpoints(hosts, []string{"127.0.0.1"})
	assert.Len(t, endpoints, 2)
	for _, ep := range endpoints {
		assert.ElementsMatch(t, ep.Targets, []string{"127.0.0.1"})
		assert.Equal(t, ep.RecordTTL, endpoint.TTL(250))
		assert.Equal(t, ep.RecordType, "A")
		assert.Contains(t, hosts, ep.DNSName)
	}

	endpoints = integration.endpoints(hosts, []string{"2001:db8::1"})
	assert.Len(t, endpoints, 2)
	for _, ep := range endpoints {
		assert.ElementsMatch(t, ep.Targets, []string{"2001:db8::1"})
		assert.Equal(t, ep.RecordTTL, endpoint.TTL(250))
		assert.Equal(t, ep.RecordType, "AAAA")
		assert.Contains(t, hosts, ep.DNSName)
	}

	endpoints = integration.endpoints(hosts, []string{"127.0.0.1", "2001:db8::1"})
	assert.Len(t, endpoints, 4)
	for _, ep := range endpoints {
		if ep.RecordType == "A" {
			assert.ElementsMatch(t, ep.Targets, []string{"127.0.0.1"})
			assert.Equal(t, ep.RecordTTL, endpoint.TTL(250))
			assert.Contains(t, hosts, ep.DNSName)
		} else {
			assert.ElementsMatch(t, ep.Targets, []string{"2001:db8::1"})
			assert.Equal(t, ep.RecordTTL, endpoint.TTL(250))
			assert.Equal(t, ep.RecordType, "AAAA")
			assert.Contains(t, hosts, ep.DNSName)
		}
	}

	endpoints = integration.endpoints(hosts, []string{"example.lb.identifier.amazonaws.com"})
	assert.Len(t, endpoints, 2)
	for _, ep := range endpoints {
		assert.ElementsMatch(t, ep.Targets, []string{"example.lb.identifier.amazonaws.com"})
		assert.Equal(t, ep.RecordTTL, endpoint.TTL(250))
		assert.Equal(t, ep.RecordType, "CNAME")
		assert.Contains(t, hosts, ep.DNSName)
	}
}

func TestExternalDNSRecordType(t *testing.T) {
	integration := externalDNS{ttl: 250}
	assert.Equal(t, "A", integration.recordType("127.0.0.1"))
	assert.Equal(t, "AAAA", integration.recordType("::FFFF:C0A8:1"))
	assert.Equal(t, "AAAA", integration.recordType("2001:db8::1"))
	assert.Equal(t, "CNAME", integration.recordType("example.lb.identifier.amazonaws.com"))
}

func TestExternalDNSWithCustomTTL(t *testing.T) {
	// Test with custom TTL
	customTTL := int64(3600)
	integration := NewExternalDNS(nil, switchboard.NewServiceTarget("my-name", "my-namespace"), &customTTL)
	externalDNSImpl := integration.(*externalDNS)
	assert.Equal(t, endpoint.TTL(3600), externalDNSImpl.ttl)

	// Test with default TTL (nil)
	integration = NewExternalDNS(nil, switchboard.NewServiceTarget("my-name", "my-namespace"), nil)
	externalDNSImpl = integration.(*externalDNS)
	assert.Equal(t, endpoint.TTL(300), externalDNSImpl.ttl)
}

//-------------------------------------------------------------------------------------------------
// UTILS
//-------------------------------------------------------------------------------------------------

func getDNSEndpoints(
	ctx context.Context, t *testing.T, ctrlClient client.Client, namespace string,
) map[string][]string {
	var list externaldnsv1alpha1.DNSEndpointList
	err := ctrlClient.List(ctx, &list, &client.ListOptions{
		Namespace: namespace,
	})
	require.Nil(t, err)

	result := make(map[string][]string)
	for _, item := range list.Items {
		names := make([]string, 0)
		for _, ep := range item.Spec.Endpoints {
			names = append(names, ep.DNSName)
		}
		result[item.Name] = names
	}
	return result
}
