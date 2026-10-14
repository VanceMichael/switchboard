package integrations

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	managedByLabelKey    = "kubernetes.io/managed-by"
	ingressAnnotationKey = "kubernetes.io/ingress.class"
)

// IngressInfo encapsulates information extracted from ingress objects that integrations act upon.
type IngressInfo struct {
	Hosts         []string
	TLSSecretName *string
}

// Integration is an interface for any component that allows to create "derivative" Kubernetes
// resources for a Traefik ingress resources. An example is the external-dns integration which
// generates DNSEndpoint resources for IngressRoute objects.
type Integration interface {
	// Name returns a canonical name for this integration to identify it in logs.
	Name() string

	// OwnedResource returns the resource (i.e. CRD of an external tool) that this integration
	// owns. The resource should be "empty", i.e. no fields should be set.
	OwnedResource() client.Object

	// WatchedObject optionally returns a particular object whose changes require the
	// reconciliation of all resources that this integration is applied to. In contrast to
	// `OwnedResource`, this method returns a concrete object (i.e. its name and namespace must
	// set set). If the integration does not watch any resources, this method may return `nil`.
	WatchedObject() client.Object

	// IngressRouteNameForResource maps the name of a child resource back to the name of the
	// IngressRoute whose preferred child carries that name. The second return value reports
	// whether the resource name matches the preferred-name pattern at all. It is used to enqueue
	// precisely the single IngressRoute that is affected by an update or deletion of a foreign
	// resource occupying its preferred name.
	IngressRouteNameForResource(resourceName string) (ingressRouteName string, ok bool)

	// UpdateResource updates the resource that ought to be owned by the passed object. Updating
	// may entail creating the resource, updating an existing resource, or deleting the resouce.
	// All information the generated resource is derived from the integration's global
	// configuration along with the given ingress information.
	UpdateResource(ctx context.Context, owner metav1.Object, info IngressInfo) error
}
