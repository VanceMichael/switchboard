package integrations

import (
	"context"
	"fmt"
	"strings"

	"dario.cat/mergo"
	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// certificateNameSuffix is appended to the IngressRoute name to derive the preferred
// certificate name.
const certificateNameSuffix = "-tls"

type certManager struct {
	client   client.Client
	template certmanager.Certificate
}

// NewCertManager initializes a new cert-manager integration which creates certificates which use
// the provided issuer.
func NewCertManager(client client.Client, template certmanager.Certificate) Integration {
	return &certManager{client, template}
}

func (*certManager) Name() string {
	return "cert-manager"
}

func (*certManager) OwnedResource() client.Object {
	return &certmanager.Certificate{}
}

func (*certManager) WatchedObject() client.Object {
	return nil
}

func (*certManager) IngressRouteNameForResource(resourceName string) (string, bool) {
	if !strings.HasSuffix(resourceName, certificateNameSuffix) {
		return "", false
	}
	return strings.TrimSuffix(resourceName, certificateNameSuffix), true
}

func (c *certManager) UpdateResource(
	ctx context.Context, owner metav1.Object, info IngressInfo,
) error {
	// If the ingress does not specify a TLS secret name or specifies no hosts, no certificate
	// needs to be created.
	want := info.TLSSecretName != nil && len(info.Hosts) > 0

	tlsSecretName := ""
	if info.TLSSecretName != nil {
		tlsSecretName = *info.TLSSecretName
	}

	mutate := func(raw client.Object) error {
		resource := raw.(*certmanager.Certificate)

		// Meta
		if err := reconcileMetadata(
			owner, resource, c.client.Scheme(), &c.template.ObjectMeta,
		); err != nil {
			return fmt.Errorf("failed to reconcile metadata: %w", err)
		}

		// Spec
		template := c.template.Spec.DeepCopy()
		template.SecretName = tlsSecretName
		template.DNSNames = info.Hosts
		if err := mergo.Merge(&resource.Spec, template, mergo.WithOverride); err != nil {
			return fmt.Errorf("failed to reconcile specification: %w", err)
		}
		return nil
	}

	children := c.children()
	if err := children.reconcile(
		ctx, owner, c.preferredName(owner), want, mutate,
	); err != nil {
		return fmt.Errorf("cert-manager: %w", err)
	}
	return nil
}

//-------------------------------------------------------------------------------------------------
// UTILS
//-------------------------------------------------------------------------------------------------

func (*certManager) preferredName(owner metav1.Object) string {
	return fmt.Sprintf("%s%s", owner.GetName(), certificateNameSuffix)
}

func (c *certManager) children() childResources {
	return childResources{
		client: c.client,
		scheme: c.client.Scheme(),
		newObject: func(namespace, name string) client.Object {
			return &certmanager.Certificate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		},
		newList: func() client.ObjectList {
			return &certmanager.CertificateList{}
		},
		listItems: func(list client.ObjectList) []client.Object {
			items := list.(*certmanager.CertificateList).Items
			result := make([]client.Object, len(items))
			for i := range items {
				result[i] = &items[i]
			}
			return result
		},
	}
}
