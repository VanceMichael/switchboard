package switchboard

import (
	"context"
	"testing"
	"time"

	"github.com/borchero/switchboard/internal/k8tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceTargetTargets(t *testing.T) {
	// Setup
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	ctrlClient := k8tests.NewClient(t, scheme)
	namespace, shutdown := k8tests.NewNamespace(ctx, t, ctrlClient)
	defer shutdown()

	// Create a new service
	service := k8tests.DummyService("my-service", namespace, 80)
	err := ctrlClient.Create(ctx, &service)
	require.Nil(t, err)

	// Check whether we find the cluster IP
	target := NewServiceTarget(service.Name, service.Namespace)
	targets, err := target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.ElementsMatch(t, service.Spec.ClusterIPs, targets)

	// Update the service to provide a load balancer
	service.Spec.Type = "LoadBalancer"
	err = ctrlClient.Update(ctx, &service)
	require.Nil(t, err)

	// Check whether we find the load balancer IP
	time.Sleep(10 * time.Second)
	name := client.ObjectKeyFromObject(&service)
	err = ctrlClient.Get(ctx, name, &service)
	require.Nil(t, err)
	targets, err = target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.ElementsMatch(t, []string{service.Status.LoadBalancer.Ingress[0].IP}, targets)
}

func TestServiceTargetTargetsHeadlessLive(t *testing.T) {
	// Live-cluster test: a headless service (without selector, so that the endpoint slice
	// controller does not manage slices for it) sources its targets from manually managed
	// endpoint slices and withdraws them while none of the endpoints is ready.
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	ctrlClient := k8tests.NewClient(t, scheme)
	namespace, shutdown := k8tests.NewNamespace(ctx, t, ctrlClient)
	defer shutdown()

	service := v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "my-headless", Namespace: namespace},
		Spec: v1.ServiceSpec{
			ClusterIP:  v1.ClusterIPNone,
			ClusterIPs: []string{v1.ClusterIPNone},
			Ports:      []v1.ServicePort{{Port: 80, Name: "http"}},
		},
	}
	require.Nil(t, ctrlClient.Create(ctx, &service))

	target := NewServiceTarget(service.Name, service.Namespace)

	// No slices yet: no targets may be published.
	targets, err := target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.Len(t, targets, 0)

	// Ready dual-stack slices result in aggregated, deduplicated addresses.
	ipv4Slice := dummyEndpointSlice("my-headless-v4", namespace, service.Name,
		discoveryv1.AddressTypeIPv4,
		discoveryv1.Endpoint{
			Addresses:  []string{"10.0.1.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)},
		})
	ipv6Slice := dummyEndpointSlice("my-headless-v6", namespace, service.Name,
		discoveryv1.AddressTypeIPv6,
		discoveryv1.Endpoint{
			Addresses:  []string{"2001:db8::1"},
			Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)},
		})
	require.Nil(t, ctrlClient.Create(ctx, &ipv4Slice))
	require.Nil(t, ctrlClient.Create(ctx, &ipv6Slice))
	targets, err = target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.ElementsMatch(t, []string{"10.0.1.1", "2001:db8::1"}, targets)

	// Endpoint becoming unready withdraws the stale addresses.
	ipv4Slice.Endpoints[0].Conditions.Ready = boolPtr(false)
	ipv6Slice.Endpoints[0].Conditions.Ready = boolPtr(false)
	require.Nil(t, ctrlClient.Update(ctx, &ipv4Slice))
	require.Nil(t, ctrlClient.Update(ctx, &ipv6Slice))
	targets, err = target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.Len(t, targets, 0)

	// Ready again: addresses are restored.
	ipv4Slice.Endpoints[0].Conditions.Ready = boolPtr(true)
	ipv6Slice.Endpoints[0].Conditions.Ready = boolPtr(true)
	require.Nil(t, ctrlClient.Update(ctx, &ipv4Slice))
	require.Nil(t, ctrlClient.Update(ctx, &ipv6Slice))
	targets, err = target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.ElementsMatch(t, []string{"10.0.1.1", "2001:db8::1"}, targets)

	// Slice deletion removes the corresponding addresses.
	require.Nil(t, ctrlClient.Delete(ctx, &ipv4Slice))
	require.Nil(t, ctrlClient.Delete(ctx, &ipv6Slice))
	targets, err = target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.Len(t, targets, 0)
}

func TestServiceTargetTargetsFromService(t *testing.T) {
	var target serviceTarget

	service := v1.Service{
		Spec: v1.ServiceSpec{ClusterIPs: []string{"10.0.0.5"}},
	}

	// Source cluster IP
	targets := target.targetsFromService(service, nil)
	assert.ElementsMatch(t, service.Spec.ClusterIPs, targets)

	// Source multiple cluster IPs
	service.Spec.ClusterIPs = []string{"10.0.0.5", "2001:db8::1"}
	targets = target.targetsFromService(service, nil)
	assert.ElementsMatch(t, service.Spec.ClusterIPs, targets)

	// Source IP from status
	service.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{
		IP: "192.168.5.5",
	}}
	targets = target.targetsFromService(service, nil)
	assert.ElementsMatch(t, []string{"192.168.5.5"}, targets)

	// Source hostname from status
	service.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{
		Hostname: "example.lb.identifier.amazonaws.com",
	}}
	targets = target.targetsFromService(service, nil)
	assert.ElementsMatch(t, []string{"example.lb.identifier.amazonaws.com"}, targets)

	// Ensure hostname takes precedence
	service.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{
		IP:       "192.168.5.5",
		Hostname: "example.lb.identifier.amazonaws.com",
	}}
	targets = target.targetsFromService(service, nil)
	assert.ElementsMatch(t, []string{"example.lb.identifier.amazonaws.com"}, targets)

	// Ensure only one hostname
	service.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{
		Hostname: "example.lb.identifier.amazonaws.com",
	}, {
		Hostname: "example2.lb.identifier.amazonaws.com",
	}}
	targets = target.targetsFromService(service, nil)
	assert.ElementsMatch(t, []string{"example.lb.identifier.amazonaws.com"}, targets)

	// Source hostname from annotation
	service.SetAnnotations(map[string]string{
		HostnameKey: "example.identifier.amazonaws.com",
	})
	targets = target.targetsFromService(service, nil)
	assert.ElementsMatch(t, []string{"example.identifier.amazonaws.com"}, targets)

	// Ensure annotation takes precedence
	service.SetAnnotations(map[string]string{
		HostnameKey: "example.identifier.amazonaws.com",
	})
	service.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{
		IP:       "192.168.5.5",
		Hostname: "example.lb.identifier.amazonaws.com",
	}}
	targets = target.targetsFromService(service, nil)
	assert.ElementsMatch(t, []string{"example.identifier.amazonaws.com"}, targets)

	// Ensure only one hostname
	service.SetAnnotations(map[string]string{
		HostnameKey: "example.identifier.amazonaws.com, example2.identifier.amazonaws.com",
	})
	targets = target.targetsFromService(service, nil)
	assert.ElementsMatch(t, []string{"example.identifier.amazonaws.com"}, targets)

	// Headless services must never publish the `None` cluster IP: ready endpoint addresses are
	// used instead.
	headless := v1.Service{
		Spec: v1.ServiceSpec{
			ClusterIP:  v1.ClusterIPNone,
			ClusterIPs: []string{v1.ClusterIPNone},
		},
	}
	targets = target.targetsFromService(headless, []string{"10.0.0.10", "2001:db8::10"})
	assert.ElementsMatch(t, []string{"10.0.0.10", "2001:db8::10"}, targets)

	// No ready endpoints for a headless service results in no targets (DNS records withdrawn).
	targets = target.targetsFromService(headless, nil)
	assert.Len(t, targets, 0)

	// Annotation hostnames take precedence over headless endpoint addresses.
	headless.Annotations = map[string]string{HostnameKey: "example.identifier.amazonaws.com"}
	targets = target.targetsFromService(headless, []string{"10.0.0.10"})
	assert.ElementsMatch(t, []string{"example.identifier.amazonaws.com"}, targets)
	headless.Annotations = nil

	// Load balancer hostnames/IPs take precedence over headless endpoint addresses.
	headless.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{IP: "192.168.5.5"}}
	targets = target.targetsFromService(headless, []string{"10.0.0.10"})
	assert.ElementsMatch(t, []string{"192.168.5.5"}, targets)
}

func boolPtr(value bool) *bool {
	return &value
}

func dummyEndpointSlice(
	name, namespace, serviceName string, addressType discoveryv1.AddressType,
	endpoints ...discoveryv1.Endpoint,
) discoveryv1.EndpointSlice {
	return discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{discoveryv1.LabelServiceName: serviceName},
		},
		AddressType: addressType,
		Endpoints:   endpoints,
	}
}

func TestReadyAddressesFromEndpointSlices(t *testing.T) {
	const serviceName = "my-service"

	ready := func(addresses ...string) discoveryv1.Endpoint {
		return discoveryv1.Endpoint{
			Addresses:  addresses,
			Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)},
		}
	}

	// Ready IPv4 and IPv6 endpoints across multiple slices are aggregated and sorted.
	slices := []discoveryv1.EndpointSlice{
		dummyEndpointSlice("slice-v4", "ns", serviceName, discoveryv1.AddressTypeIPv4,
			ready("10.0.0.2"), ready("10.0.0.1")),
		dummyEndpointSlice("slice-v6", "ns", serviceName, discoveryv1.AddressTypeIPv6,
			ready("2001:db8::2")),
	}
	targets := ReadyAddressesFromEndpointSlices(serviceName, slices)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2", "2001:db8::2"}, targets)

	// Duplicate addresses (e.g. across slices) are deduplicated.
	slices = append(slices, dummyEndpointSlice("slice-dup", "ns", serviceName,
		discoveryv1.AddressTypeIPv4, ready("10.0.0.1", "10.0.0.3")))
	targets = ReadyAddressesFromEndpointSlices(serviceName, slices)
	assert.Equal(t,
		[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "2001:db8::2"}, targets)

	// Not-ready endpoints (ready false or nil) are skipped.
	slices = []discoveryv1.EndpointSlice{
		dummyEndpointSlice("slice-not-ready", "ns", serviceName, discoveryv1.AddressTypeIPv4,
			discoveryv1.Endpoint{
				Addresses:  []string{"10.0.0.4"},
				Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(false)},
			},
			discoveryv1.Endpoint{
				Addresses:  []string{"10.0.0.5"},
				Conditions: discoveryv1.EndpointConditions{Ready: nil},
			},
			ready("10.0.0.6"),
		),
	}
	targets = ReadyAddressesFromEndpointSlices(serviceName, slices)
	assert.Equal(t, []string{"10.0.0.6"}, targets)

	// Terminating endpoints are skipped even if their ready condition is still true.
	slices = []discoveryv1.EndpointSlice{
		dummyEndpointSlice("slice-terminating", "ns", serviceName, discoveryv1.AddressTypeIPv4,
			discoveryv1.Endpoint{
				Addresses:  []string{"10.0.0.7"},
				Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true), Terminating: boolPtr(true)},
			},
			ready("10.0.0.8"),
		),
	}
	targets = ReadyAddressesFromEndpointSlices(serviceName, slices)
	assert.Equal(t, []string{"10.0.0.8"}, targets)

	// Slices belonging to another service are ignored.
	slices = []discoveryv1.EndpointSlice{
		dummyEndpointSlice("slice-other", "ns", "other-service", discoveryv1.AddressTypeIPv4,
			ready("10.0.0.9")),
	}
	targets = ReadyAddressesFromEndpointSlices(serviceName, slices)
	assert.Len(t, targets, 0)

	// Slices with a non-IP address type and invalid addresses are ignored.
	slices = []discoveryv1.EndpointSlice{
		dummyEndpointSlice("slice-fqdn", "ns", serviceName, discoveryv1.AddressTypeFQDN,
			ready("pod.example.com")),
		dummyEndpointSlice("slice-invalid", "ns", serviceName, discoveryv1.AddressTypeIPv4,
			ready("not-an-ip", ""), ready("10.0.0.10")),
	}
	targets = ReadyAddressesFromEndpointSlices(serviceName, slices)
	assert.Equal(t, []string{"10.0.0.10"}, targets)

	// No slices at all yields no targets.
	assert.Len(t, ReadyAddressesFromEndpointSlices(serviceName, nil), 0)
}

func TestServiceTargetTargetsHeadless(t *testing.T) {
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

	// A headless service without any endpoint slices has no publishable targets.
	ctrlClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&headless).Build()
	target := NewServiceTarget(serviceName, namespace)
	targets, err := target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.Len(t, targets, 0)

	// Ready IPv4 and IPv6 addresses of the service's slices are published.
	ipv4Slice := dummyEndpointSlice("headless-v4", namespace, serviceName,
		discoveryv1.AddressTypeIPv4,
		discoveryv1.Endpoint{
			Addresses:  []string{"10.0.1.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)},
		})
	ipv6Slice := dummyEndpointSlice("headless-v6", namespace, serviceName,
		discoveryv1.AddressTypeIPv6,
		discoveryv1.Endpoint{
			Addresses:  []string{"2001:db8::1"},
			Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)},
		})
	otherSlice := dummyEndpointSlice("other-v4", namespace, "other-service",
		discoveryv1.AddressTypeIPv4,
		discoveryv1.Endpoint{
			Addresses:  []string{"10.0.9.9"},
			Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)},
		})
	require.Nil(t, ctrlClient.Create(ctx, &ipv4Slice))
	require.Nil(t, ctrlClient.Create(ctx, &ipv6Slice))
	require.Nil(t, ctrlClient.Create(ctx, &otherSlice))
	targets, err = target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.Equal(t, []string{"10.0.1.1", "2001:db8::1"}, targets)

	// When the only endpoints become not ready, no addresses remain.
	ipv4Slice.Endpoints[0].Conditions.Ready = boolPtr(false)
	ipv6Slice.Endpoints[0].Conditions.Ready = boolPtr(false)
	require.Nil(t, ctrlClient.Update(ctx, &ipv4Slice))
	require.Nil(t, ctrlClient.Update(ctx, &ipv6Slice))
	targets, err = target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.Len(t, targets, 0)

	// Newly available addresses are picked up automatically.
	ipv4Slice.Endpoints[0].Conditions.Ready = boolPtr(true)
	ipv4Slice.Endpoints = append(ipv4Slice.Endpoints, discoveryv1.Endpoint{
		Addresses:  []string{"10.0.1.2"},
		Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)},
	})
	require.Nil(t, ctrlClient.Update(ctx, &ipv4Slice))
	targets, err = target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.Equal(t, []string{"10.0.1.1", "10.0.1.2"}, targets)

	// Slice deletion no longer publishes the removed addresses.
	require.Nil(t, ctrlClient.Delete(ctx, &ipv4Slice))
	targets, err = target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.Len(t, targets, 0)

	// Switching the service back to a regular address source falls back to the cluster IP.
	var updated v1.Service
	require.Nil(t, ctrlClient.Get(ctx,
		client.ObjectKey{Name: serviceName, Namespace: namespace}, &updated))
	updated.Spec.ClusterIP = "10.96.0.1"
	updated.Spec.ClusterIPs = []string{"10.96.0.1"}
	require.Nil(t, ctrlClient.Update(ctx, &updated))
	targets, err = target.Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.Equal(t, []string{"10.96.0.1"}, targets)
}

func TestServiceTargetTargetsHeadlessPrecedence(t *testing.T) {
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	const namespace = "ns"
	const serviceName = "headless"

	headless := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Annotations: map[string]string{
				HostnameKey: "example.identifier.amazonaws.com",
			},
		},
		Spec: v1.ServiceSpec{
			ClusterIP:  v1.ClusterIPNone,
			ClusterIPs: []string{v1.ClusterIPNone},
			Ports:      []v1.ServicePort{{Port: 80, Name: "http"}},
		},
	}
	slice := dummyEndpointSlice("headless-v4", namespace, serviceName,
		discoveryv1.AddressTypeIPv4,
		discoveryv1.Endpoint{
			Addresses:  []string{"10.0.1.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(true)},
		})
	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).WithObjects(&headless, &slice).Build()

	targets, err := NewServiceTarget(serviceName, namespace).Targets(ctx, ctrlClient)
	require.Nil(t, err)
	assert.Equal(t, []string{"example.identifier.amazonaws.com"}, targets)
}

func TestServiceTargetNamespacedName(t *testing.T) {
	target := NewServiceTarget("my-service", "my-namespace")
	name := target.NamespacedName()
	assert.Equal(t, "my-service", name.Name)
	assert.Equal(t, "my-namespace", name.Namespace)
}

func TestStaticTargetIPs(t *testing.T) {
	ctx := context.Background()
	expectedIPs := []string{"127.0.0.1", "2001:db8::1"}
	target := NewStaticTarget(expectedIPs...)
	ips, err := target.Targets(ctx, nil)
	require.Nil(t, err)
	assert.ElementsMatch(t, expectedIPs, ips)
}
