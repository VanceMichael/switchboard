package switchboard

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const HostnameKey = "external-dns.alpha.kubernetes.io/hostname"

// Target is a type which allows to retrieve a potentially dynamically changing IP from Kubernetes.
type Target interface {
	// Targets returns the IPv4/IPv6 addresses or hostnames that should be used as targets or an
	// error if the addresses/hostnames cannot be retrieved.
	Targets(ctx context.Context, client client.Client) ([]string, error)
	// NamespacedName returns the namespaced name of the dynamic target service or none if the IP
	// is not retrieved dynamically.
	NamespacedName() *types.NamespacedName
}

//-------------------------------------------------------------------------------------------------
// ANNOTATION HOSTNAME
//-------------------------------------------------------------------------------------------------

// SplitHostnameAnnotation splits a comma-separated hostname annotation string into a slice of hostnames.
// It trims any leading or trailing whitespace and removes any spaces within the anno
func SplitHostnameAnnotation(input string) []string {
	return strings.Split(strings.TrimSpace(strings.ReplaceAll(input, " ", "")), ",")
}

func HostnamesFromAnnotations(input map[string]string) []string {
	return extractHostnamesFromAnnotations(input, HostnameKey)
}

func extractHostnamesFromAnnotations(input map[string]string, key string) []string {
	annotation, ok := input[key]
	if !ok {
		return nil
	}
	return SplitHostnameAnnotation(annotation)
}

//-------------------------------------------------------------------------------------------------
// SERVICE TARGET
//-------------------------------------------------------------------------------------------------

type serviceTarget struct {
	name types.NamespacedName
}

// NewServiceTarget creates a new target which dynamically sources the IP from the provided
// Kubernetes service.
func NewServiceTarget(name, namespace string) Target {
	return serviceTarget{
		name: types.NamespacedName{Name: name, Namespace: namespace},
	}
}

func (t serviceTarget) Targets(ctx context.Context, ctrlClient client.Client) ([]string, error) {
	// Get service
	var service v1.Service
	if err := ctrlClient.Get(ctx, t.name, &service); err != nil {
		return nil, fmt.Errorf("failed to query service: %w", err)
	}

	// For headless services, the ClusterIP is the sentinel `None` which is not a routable
	// address. Instead, we aggregate the ready addresses of the service's endpoint slices.
	var headlessTargets []string
	if isHeadlessService(&service) {
		var slices discoveryv1.EndpointSliceList
		if err := ctrlClient.List(ctx, &slices,
			client.InNamespace(service.Namespace),
			client.MatchingLabels{discoveryv1.LabelServiceName: service.Name},
		); err != nil {
			return nil, fmt.Errorf("failed to list endpoint slices for service %q: %w",
				t.name.String(), err)
		}
		headlessTargets = ReadyAddressesFromEndpointSlices(service.Name, slices.Items)
	}
	return t.targetsFromService(service, headlessTargets), nil
}

// isHeadlessService reports whether the given service is headless, i.e. it does not get a
// cluster IP assigned. Kubernetes represents the absence of a cluster IP using the sentinel
// value `None`.
func isHeadlessService(service *v1.Service) bool {
	if service.Spec.ClusterIP == v1.ClusterIPNone {
		return true
	}
	for _, ip := range service.Spec.ClusterIPs {
		if ip == v1.ClusterIPNone {
			return true
		}
	}
	return false
}

// ReadyAddressesFromEndpointSlices aggregates the unique, ready IPv4/IPv6 addresses exposed by
// the endpoint slices that belong to the provided service. Addresses of endpoints that are not
// ready, that are terminating, that are not valid IPs, or that are served by endpoint slices of
// another service are skipped. The returned addresses are sorted to yield stable results.
func ReadyAddressesFromEndpointSlices(
	serviceName string, slices []discoveryv1.EndpointSlice,
) []string {
	targets := make([]string, 0)
	seen := make(map[string]struct{})
	for _, slice := range slices {
		// Only accept slices that belong to the target service. Although callers typically
		// already filter by the `kubernetes.io/service-name` label, we re-check defensively to
		// ensure endpoints of other services never leak into the result.
		if slice.Labels[discoveryv1.LabelServiceName] != serviceName {
			continue
		}
		if slice.AddressType != discoveryv1.AddressTypeIPv4 &&
			slice.AddressType != discoveryv1.AddressTypeIPv6 {
			continue
		}
		for _, endpoint := range slice.Endpoints {
			if !isReadyEndpoint(&endpoint) {
				continue
			}
			for _, address := range endpoint.Addresses {
				if address == "" || net.ParseIP(address) == nil {
					continue
				}
				if _, ok := seen[address]; ok {
					continue
				}
				seen[address] = struct{}{}
				targets = append(targets, address)
			}
		}
	}
	sort.Strings(targets)
	return targets
}

// isReadyEndpoint reports whether the given endpoint slice endpoint may serve traffic: its ready
// condition must explicitly be `true` and it must not be terminating.
func isReadyEndpoint(endpoint *discoveryv1.Endpoint) bool {
	if endpoint.Conditions.Ready == nil || !*endpoint.Conditions.Ready {
		return false
	}
	if endpoint.Conditions.Terminating != nil && *endpoint.Conditions.Terminating {
		return false
	}
	return true
}

// targetsFromService resolves the DNS targets of a service using the documented precedence:
//  1. hostnames set via the `external-dns.alpha.kubernetes.io/hostname` annotation,
//  2. hostnames/IPs reported via the load balancer ingress status,
//  3. the ready endpoint addresses for headless services (which may be empty, signaling that
//     stale DNS records must be withdrawn),
//  4. the service's cluster IPs.
func (serviceTarget) targetsFromService(
	service v1.Service, headlessTargets []string,
) []string {
	targets := make([]string, 0)

	// Prioritize annotation hostnames.
	hostnameList := HostnamesFromAnnotations(service.Annotations)
	for _, hostname := range hostnameList {
		// We cannot have more than one CNAME record, the annotation overwrites everything
		targets = []string{hostname}
		break
	}

	// Try to get load balancer IPs/hostnames...
	if len(targets) == 0 {
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			if ingress.Hostname != "" {
				// We cannot have more than one CNAME record, the hostname overwrites everything
				targets = []string{ingress.Hostname}
				break
			}
			if ingress.IP != "" {
				targets = append(targets, ingress.IP)
			}
		}
	}

	if len(targets) == 0 {
		if isHeadlessService(&service) {
			// For headless services, the cluster IP is the `None` sentinel and must never be
			// published. An empty slice is intentional: it causes the DNSEndpoint to be removed
			// until ready endpoints become available again.
			targets = append(targets, headlessTargets...)
		} else {
			// ...fall back to cluster IPs
			targets = append(targets, service.Spec.ClusterIPs...)
		}
	}
	return targets
}

func (t serviceTarget) NamespacedName() *types.NamespacedName {
	return &t.name
}

//-------------------------------------------------------------------------------------------------
// STATIC TARGET
//-------------------------------------------------------------------------------------------------

type staticTarget struct {
	ips []string
}

// NewStaticTarget creates a new target which provides the given static IPs. IPs may be IPv4 or
// IPv6 addresses (and any combination thereof).
func NewStaticTarget(ips ...string) Target {
	return staticTarget{ips}
}

func (t staticTarget) Targets(ctx context.Context, client client.Client) ([]string, error) {
	return t.ips, nil
}

func (staticTarget) NamespacedName() *types.NamespacedName {
	return nil
}
