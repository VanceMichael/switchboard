package integrations

import (
	"context"
	"fmt"

	"github.com/asaskevich/govalidator"
	"github.com/borchero/switchboard/internal/switchboard"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	externaldnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"
)

type externalDNS struct {
	client client.Client
	target switchboard.Target
	ttl    endpoint.TTL
}

// NewExternalDNS initializes a new external-dns integration whose created DNS endpoints target the
// provided service. If ttl is nil, a default TTL of 300 seconds is used.
func NewExternalDNS(client client.Client, target switchboard.Target, ttl *int64) Integration {
	ttlValue := endpoint.TTL(300)
	if ttl != nil {
		ttlValue = endpoint.TTL(*ttl)
	}
	return &externalDNS{client, target, ttlValue}
}

func (*externalDNS) Name() string {
	return "external-dns"
}

func (*externalDNS) OwnedResource() client.Object {
	return &externaldnsv1alpha1.DNSEndpoint{}
}

func (e *externalDNS) WatchedObject() client.Object {
	name := e.target.NamespacedName()
	if name == nil {
		return nil
	}
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}
}

func (*externalDNS) IngressRouteNameForResource(resourceName string) (string, bool) {
	// The preferred DNSEndpoint name is identical to the IngressRoute name. Whether an
	// IngressRoute of that name exists is verified by the enqueue mapper via a lookup.
	return resourceName, true
}

func (e *externalDNS) UpdateResource(
	ctx context.Context, owner metav1.Object, info IngressInfo,
) error {
	// If the ingress specifies no hosts, there should be no endpoint.
	want := len(info.Hosts) > 0

	var targets []string
	if want {
		var err error
		targets, err = e.target.Targets(ctx, e.client)
		if err != nil {
			return fmt.Errorf("failed to query IP for DNS A record: %w", err)
		}
	}

	mutate := func(raw client.Object) error {
		resource := raw.(*externaldnsv1alpha1.DNSEndpoint)

		// Preserve the historical behavior: newly created endpoints inherit the annotations of
		// the owner.
		if resource.GetResourceVersion() == "" {
			resource.SetAnnotations(cloneAnnotations(owner.GetAnnotations()))
		}

		// Meta
		if err := reconcileMetadata(owner, resource, e.client.Scheme()); err != nil {
			return fmt.Errorf("failed to reconcile metadata: %w", err)
		}

		// Spec
		resource.Spec.Endpoints = e.endpoints(info.Hosts, targets)
		return nil
	}

	children := e.children()
	if err := children.reconcile(ctx, owner, owner.GetName(), want, mutate); err != nil {
		return fmt.Errorf("external-dns: %w", err)
	}
	return nil
}

//-------------------------------------------------------------------------------------------------
// UTILS
//-------------------------------------------------------------------------------------------------

func (e *externalDNS) children() childResources {
	return childResources{
		client: e.client,
		scheme: e.client.Scheme(),
		newObject: func(namespace, name string) client.Object {
			return &externaldnsv1alpha1.DNSEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		},
		newList: func() client.ObjectList {
			return &externaldnsv1alpha1.DNSEndpointList{}
		},
		listItems: func(list client.ObjectList) []client.Object {
			items := list.(*externaldnsv1alpha1.DNSEndpointList).Items
			result := make([]client.Object, len(items))
			for i := range items {
				result[i] = &items[i]
			}
			return result
		},
	}
}

func (e *externalDNS) endpoints(hosts []string, targets []string) []*endpoint.Endpoint {
	// Get the records for the target service
	targetRecords := make(map[string][]string)
	for _, target := range targets {
		rtype := e.recordType(target)
		targetRecords[rtype] = append(targetRecords[rtype], target)
	}

	// Create the endpoints
	endpoints := make([]*endpoint.Endpoint, 0, len(hosts))
	for _, host := range hosts {
		for rtype, values := range targetRecords {
			endpoints = append(endpoints, &endpoint.Endpoint{
				DNSName:    host,
				Targets:    values,
				RecordType: rtype,
				RecordTTL:  e.ttl,
			})
		}
	}
	return endpoints
}

func (*externalDNS) recordType(target string) string {
	if govalidator.IsIPv4(target) {
		return "A"
	}
	if govalidator.IsIPv6(target) {
		return "AAAA"
	}
	return "CNAME"
}
