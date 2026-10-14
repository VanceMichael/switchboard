package integrations

import (
	"context"
	"regexp"
	"testing"

	"github.com/borchero/switchboard/internal/k8tests"
	"github.com/borchero/switchboard/internal/switchboard"
	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	externaldnsv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

const ownershipTestNamespace = "ownership-test"

// dns1123Subdomain matches legal Kubernetes resource names (RFC 1123 subdomain).
var dns1123Subdomain = regexp.MustCompile(
	`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`,
)

//-------------------------------------------------------------------------------------------------
// PURE HELPERS
//-------------------------------------------------------------------------------------------------

func TestAlternativeName(t *testing.T) {
	preferred := "my-ingress-tls"
	uid := types.UID("owner-uid-1")

	// Deterministic and stable
	alt1 := AlternativeName(preferred, uid)
	assert.Equal(t, alt1, AlternativeName(preferred, uid))

	// Legal name and bounded length
	assert.Regexp(t, dns1123Subdomain, alt1)
	assert.LessOrEqual(t, len(alt1), resourceNameMaxLength)

	// Distinct UIDs (e.g. a recreated same-named owner) map to distinct alternatives
	assert.NotEqual(t, alt1, AlternativeName(preferred, types.UID("owner-uid-2")))

	// The alternative must not itself match the preferred-name pattern used for certificates
	// (it ends with the UID hash, not with "-tls").
	assert.False(t, hasSuffix(alt1, certificateNameSuffix))

	// Long preferred names are truncated into a legal alternative
	altLong := AlternativeName(repeat("a", 300)+certificateNameSuffix, uid)
	assert.LessOrEqual(t, len(altLong), resourceNameMaxLength)
	assert.Regexp(t, dns1123Subdomain, altLong)

	// Trailing dashes are absorbed so the suffix separator is legal
	assert.Regexp(t, dns1123Subdomain, AlternativeName("dash-", uid))
}

func TestControlledByPredicate(t *testing.T) {
	// No controller reference at all
	unowned := k8tests.DummyService("other", ownershipTestNamespace, 80)
	assert.False(t, controlledBy(&unowned, "Service", "my-ingress", "uid-1"))

	// Matching controller reference
	owned := k8tests.DummyService("child", ownershipTestNamespace, 80)
	owned.OwnerReferences = []metav1.OwnerReference{
		controllerRef("Service", "v1", "my-ingress", "uid-1"),
	}
	assert.True(t, controlledBy(&owned, "Service", "my-ingress", "uid-1"))

	// Another controller kind
	foreign := owned.DeepCopy()
	foreign.OwnerReferences = []metav1.OwnerReference{
		controllerRef("Deployment", "apps/v1", "my-ingress", "uid-1"),
	}
	assert.False(t, controlledBy(foreign, "Service", "my-ingress", "uid-1"))

	// Stale UID: same kind and name but a different UID (owner recreated)
	stale := owned.DeepCopy()
	stale.OwnerReferences = []metav1.OwnerReference{
		controllerRef("Service", "v1", "my-ingress", "uid-stale"),
	}
	assert.False(t, controlledBy(stale, "Service", "my-ingress", "uid-1"))

	// A non-controller reference does not establish ownership
	ref := controllerRef("Service", "v1", "my-ingress", "uid-1")
	ref.Controller = nil
	weak := k8tests.DummyService("child", ownershipTestNamespace, 80)
	weak.OwnerReferences = []metav1.OwnerReference{ref}
	assert.False(t, controlledBy(&weak, "Service", "my-ingress", "uid-1"))
}

func TestIngressRouteNameForResource(t *testing.T) {
	cert := NewCertManager(nil, certmanager.Certificate{}).(*certManager)
	name, ok := cert.IngressRouteNameForResource("my-ingress-tls")
	assert.True(t, ok)
	assert.Equal(t, "my-ingress", name)
	_, ok = cert.IngressRouteNameForResource("my-ingress")
	assert.False(t, ok)
	// UID-hash alternative names do not invert to a route
	_, ok = cert.IngressRouteNameForResource(
		AlternativeName("my-ingress-tls", types.UID("uid-1")),
	)
	assert.False(t, ok)

	dns := NewExternalDNS(nil, switchboard.NewStaticTarget(), nil).(*externalDNS)
	name, ok = dns.IngressRouteNameForResource("my-ingress")
	assert.True(t, ok)
	assert.Equal(t, "my-ingress", name)
}

//-------------------------------------------------------------------------------------------------
// CERTIFICATE INTEGRATION
//-------------------------------------------------------------------------------------------------

func TestCertPreferredLifecycleWithoutConflict(t *testing.T) {
	ctx := context.Background()
	owner := ownershipOwner(map[string]string{"team": "platform"})
	cl := newOwnershipFakeClient(owner)
	integration := certIntegration(cl)

	// Create at the preferred name
	require.Nil(t, integration.UpdateResource(ctx, owner, certInfo()))
	cert := mustGetCert(t, ctx, cl, "my-ingress-tls")
	assert.Equal(t, []metav1.OwnerReference{
		controllerRef("Service", "v1", "my-ingress", "owner-uid-1"),
	}, cert.OwnerReferences)
	assert.Equal(t, "switchboard", cert.Labels[managedByLabelKey])
	assert.Equal(t, "tls-secret", cert.Spec.SecretName)
	assert.Equal(t, []string{"example.com"}, cert.Spec.DNSNames)
	assert.Len(t, listCerts(ctx, t, cl), 1)

	// Update patches in place
	info := certInfo()
	info.Hosts = []string{"example.com", "www.example.com"}
	require.Nil(t, integration.UpdateResource(ctx, owner, info))
	cert = mustGetCert(t, ctx, cl, "my-ingress-tls")
	assert.ElementsMatch(t, info.Hosts, cert.Spec.DNSNames)
	assert.Len(t, listCerts(ctx, t, cl), 1)

	// Cleanup removes the owned child
	require.Nil(t, integration.UpdateResource(ctx, owner, IngressInfo{}))
	assert.True(t, certNotFound(ctx, t, cl, "my-ingress-tls"))
}

func TestCertForeignPreferredUsesStableAlternative(t *testing.T) {
	ctx := context.Background()
	owner := ownershipOwner(nil)
	cl := newOwnershipFakeClient(owner)

	// Platform team pre-reserves the preferred name without a controller reference.
	foreign := &certmanager.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress-tls",
			Namespace: ownershipTestNamespace,
			Labels:    map[string]string{"owner": "platform"},
		},
		Spec: certmanager.CertificateSpec{SecretName: "handcrafted"},
	}
	require.Nil(t, cl.Create(ctx, foreign))

	integration := certIntegration(cl)
	require.Nil(t, integration.UpdateResource(ctx, owner, certInfo()))

	certs := listCerts(ctx, t, cl)
	require.Len(t, certs, 2)

	// Foreign object is preserved untouched.
	preserved := mustGetCert(t, ctx, cl, "my-ingress-tls")
	assert.Equal(t, map[string]string{"owner": "platform"}, preserved.Labels)
	assert.Empty(t, preserved.OwnerReferences)
	assert.Equal(t, "handcrafted", preserved.Spec.SecretName)

	// Our certificate lives at the deterministic alternative name.
	altName := AlternativeName("my-ingress-tls", owner.UID)
	alt := mustGetCert(t, ctx, cl, altName)
	assert.Equal(t, []metav1.OwnerReference{
		controllerRef("Service", "v1", "my-ingress", "owner-uid-1"),
	}, alt.OwnerReferences)
	assert.Equal(t, "switchboard", alt.Labels[managedByLabelKey])
	assert.Equal(t, "tls-secret", alt.Spec.SecretName)

	// Reconciling again is stable: same alternative name, no extras.
	require.Nil(t, integration.UpdateResource(ctx, owner, certInfo()))
	assert.Len(t, listCerts(ctx, t, cl), 2)
	assert.False(t, certNotFound(ctx, t, cl, altName))
}

func TestCertForeignControllersAndStaleUID(t *testing.T) {
	ctx := context.Background()
	owner := ownershipOwner(nil)
	cl := newOwnershipFakeClient(owner)

	// Another controller occupies the preferred name.
	other := &certmanager.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress-tls",
			Namespace: ownershipTestNamespace,
			OwnerReferences: []metav1.OwnerReference{
				controllerRef("Deployment", "apps/v1", "someone-else", "other-uid"),
			},
		},
	}
	require.Nil(t, cl.Create(ctx, other))
	integration := certIntegration(cl)
	require.Nil(t, integration.UpdateResource(ctx, owner, certInfo()))

	altName := AlternativeName("my-ingress-tls", owner.UID)
	assert.False(t, certNotFound(ctx, t, cl, altName))
	preserved := mustGetCert(t, ctx, cl, "my-ingress-tls")
	require.Len(t, preserved.OwnerReferences, 1)
	assert.Equal(t, "Deployment", preserved.OwnerReferences[0].Kind)

	// Stale UID: same kind/name but a previous incarnation of the owner.
	require.Nil(t, cl.Delete(ctx, other))
	stale := &certmanager.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress-tls",
			Namespace: ownershipTestNamespace,
			OwnerReferences: []metav1.OwnerReference{
				controllerRef("Service", "v1", "my-ingress", "owner-uid-stale"),
			},
		},
	}
	require.Nil(t, cl.Create(ctx, stale))
	require.Nil(t, integration.UpdateResource(ctx, owner, certInfo()))
	assert.False(t, certNotFound(ctx, t, cl, altName))
	assert.Equal(t,
		types.UID("owner-uid-stale"),
		mustGetCert(t, ctx, cl, "my-ingress-tls").OwnerReferences[0].UID,
	)
}

func TestCertMigrationBackToPreferred(t *testing.T) {
	ctx := context.Background()
	owner := ownershipOwner(nil)
	cl := newOwnershipFakeClient(owner)
	altName := AlternativeName("my-ingress-tls", owner.UID)

	// Start with a foreign reservation and reconcile into the alternative.
	foreign := &certmanager.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress-tls",
			Namespace: ownershipTestNamespace,
		},
	}
	require.Nil(t, cl.Create(ctx, foreign))
	require.Nil(t, certIntegration(cl).UpdateResource(ctx, owner, certInfo()))
	assert.False(t, certNotFound(ctx, t, cl, altName))

	// The preferred name frees up: the preferred child must be created before the alternative
	// is deleted.
	require.Nil(t, cl.Delete(ctx, foreign))
	var ops []string
	tracking := &opTrackingClient{Client: cl, ops: &ops}
	require.Nil(t, certIntegration(tracking).UpdateResource(ctx, owner, certInfo()))

	createIdx := indexOf(ops, "create:my-ingress-tls")
	deleteIdx := indexOf(ops, "delete:"+altName)
	require.GreaterOrEqual(t, createIdx, 0)
	require.GreaterOrEqual(t, deleteIdx, 0)
	assert.Less(t, createIdx, deleteIdx, "preferred must be created before alternative is deleted")

	migrated := mustGetCert(t, ctx, cl, "my-ingress-tls")
	assert.Equal(t, owner.UID, migrated.OwnerReferences[0].UID)
	assert.Equal(t, "tls-secret", migrated.Spec.SecretName)
	assert.True(t, certNotFound(ctx, t, cl, altName))
	assert.Len(t, listCerts(ctx, t, cl), 1)
}

func TestCertPreferredFailureKeepsAlternative(t *testing.T) {
	ctx := context.Background()
	owner := ownershipOwner(nil)
	cl := newOwnershipFakeClient(owner)
	altName := AlternativeName("my-ingress-tls", owner.UID)

	// Occupy the preferred name and reconcile into the alternative.
	foreign := &certmanager.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress-tls",
			Namespace: ownershipTestNamespace,
		},
	}
	require.Nil(t, cl.Create(ctx, foreign))
	require.Nil(t, certIntegration(cl).UpdateResource(ctx, owner, certInfo()))

	// The preferred name frees up, but creating it now fails: the alternative must remain in
	// place and the returned error must be retryable.
	require.Nil(t, cl.Delete(ctx, foreign))
	failing := &createFailingClient{
		Client: cl,
		groupResource: schema.GroupResource{
			Group: "cert-manager.io", Resource: "certificates",
		},
		failName: "my-ingress-tls",
	}
	err := certIntegration(failing).UpdateResource(ctx, owner, certInfo())
	require.Error(t, err)
	assert.True(t, apierrors.IsAlreadyExists(err))

	alt := mustGetCert(t, ctx, cl, altName)
	assert.Equal(t, owner.UID, alt.OwnerReferences[0].UID)
	assert.Equal(t, "tls-secret", alt.Spec.SecretName)
	assert.True(t, certNotFound(ctx, t, cl, "my-ingress-tls"))

	// Once the preferred can be created, migration succeeds without a gap.
	require.Nil(t, certIntegration(cl).UpdateResource(ctx, owner, certInfo()))
	assert.False(t, certNotFound(ctx, t, cl, "my-ingress-tls"))
	assert.True(t, certNotFound(ctx, t, cl, altName))
}

func TestCertCleanupPreservesForeignObjects(t *testing.T) {
	ctx := context.Background()
	owner := ownershipOwner(nil)
	cl := newOwnershipFakeClient(owner)
	altName := AlternativeName("my-ingress-tls", owner.UID)

	foreign := &certmanager.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress-tls",
			Namespace: ownershipTestNamespace,
			Labels:    map[string]string{"owner": "platform"},
		},
	}
	require.Nil(t, cl.Create(ctx, foreign))
	integration := certIntegration(cl)
	require.Nil(t, integration.UpdateResource(ctx, owner, certInfo()))
	assert.False(t, certNotFound(ctx, t, cl, altName))

	// TLS/hosts disappear: only the UID-owned alternative is removed.
	require.Nil(t, integration.UpdateResource(ctx, owner, IngressInfo{}))
	preserved := mustGetCert(t, ctx, cl, "my-ingress-tls")
	assert.Equal(t, "platform", preserved.Labels["owner"])
	assert.Empty(t, preserved.OwnerReferences)
	assert.True(t, certNotFound(ctx, t, cl, altName))
}

//-------------------------------------------------------------------------------------------------
// DNS ENDPOINT INTEGRATION
//-------------------------------------------------------------------------------------------------

func TestDNSEndpointConflictAndMigration(t *testing.T) {
	ctx := context.Background()
	owner := ownershipOwner(map[string]string{"team": "platform"})
	cl := newOwnershipFakeClient(owner)
	altName := AlternativeName("my-ingress", owner.UID)

	// Foreign, unmanaged endpoint at the preferred name.
	foreign := &externaldnsv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: ownershipTestNamespace,
			Labels:    map[string]string{"owner": "platform"},
		},
	}
	require.Nil(t, cl.Create(ctx, foreign))

	integration := dnsIntegration(cl)
	info := IngressInfo{Hosts: []string{"example.com"}}
	require.Nil(t, integration.UpdateResource(ctx, owner, info))

	// Foreign endpoint preserved, alternative owned and functional.
	preserved := mustGetEndpoint(t, ctx, cl, "my-ingress")
	assert.Empty(t, preserved.OwnerReferences)
	assert.Equal(t, "platform", preserved.Labels["owner"])

	alt := mustGetEndpoint(t, ctx, cl, altName)
	assert.Equal(t, owner.UID, alt.OwnerReferences[0].UID)
	assert.Equal(t, "switchboard", alt.Labels[managedByLabelKey])
	require.Len(t, alt.Spec.Endpoints, 1)
	assert.Equal(t, "example.com", alt.Spec.Endpoints[0].DNSName)
	assert.Equal(t, []string{"127.0.0.1"}, []string(alt.Spec.Endpoints[0].Targets))

	// Stable across repeated reconciles.
	require.Nil(t, integration.UpdateResource(ctx, owner, info))
	assert.Len(t, listEndpoints(ctx, t, cl), 2)
	assert.False(t, endpointNotFound(ctx, t, cl, altName))

	// Preferred frees up: migrate back.
	require.Nil(t, cl.Delete(ctx, foreign))
	require.Nil(t, integration.UpdateResource(ctx, owner, info))
	migrated := mustGetEndpoint(t, ctx, cl, "my-ingress")
	assert.Equal(t, owner.UID, migrated.OwnerReferences[0].UID)
	// Compatibility: newly created endpoints inherit the owner's annotations and labels.
	assert.Equal(t, "platform", migrated.Annotations["team"])
	assert.True(t, endpointNotFound(ctx, t, cl, altName))

	// Cleanup with hosts gone while a foreign object occupies the preferred name: the foreign
	// object stays and no alternative is created.
	var currentPreferred externaldnsv1alpha1.DNSEndpoint
	require.Nil(t, cl.Get(ctx, types.NamespacedName{
		Namespace: ownershipTestNamespace,
		Name:      "my-ingress",
	}, &currentPreferred))
	require.Nil(t, cl.Delete(ctx, &currentPreferred))
	require.Nil(t, cl.Create(ctx, &externaldnsv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: ownershipTestNamespace,
			Labels:    map[string]string{"owner": "platform"},
		},
	}))
	require.Nil(t, integration.UpdateResource(ctx, owner, IngressInfo{}))
	preserved = mustGetEndpoint(t, ctx, cl, "my-ingress")
	assert.Empty(t, preserved.OwnerReferences)
	assert.Equal(t, "platform", preserved.Labels["owner"])
	assert.True(t, endpointNotFound(ctx, t, cl, altName))
}

//-------------------------------------------------------------------------------------------------
// HELPERS
//-------------------------------------------------------------------------------------------------

func ownershipOwner(annotations map[string]string) *v1.Service {
	owner := k8tests.DummyService("my-ingress", ownershipTestNamespace, 80)
	owner.UID = types.UID("owner-uid-1")
	owner.Annotations = annotations
	return &owner
}

func newOwnershipFakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(k8tests.NewScheme()).
		WithObjects(objs...).
		Build()
}

func certIntegration(cl client.Client) Integration {
	return NewCertManager(cl, certmanager.Certificate{
		Spec: certmanager.CertificateSpec{
			IssuerRef: cmmeta.IssuerReference{
				Kind: "ClusterIssuer",
				Name: "my-issuer",
			},
		},
	})
}

func dnsIntegration(cl client.Client) Integration {
	return NewExternalDNS(cl, switchboard.NewStaticTarget("127.0.0.1"), nil)
}

func certInfo() IngressInfo {
	secret := "tls-secret"
	return IngressInfo{Hosts: []string{"example.com"}, TLSSecretName: &secret}
}

func controllerRef(kind, apiVersion, name string, uid types.UID) metav1.OwnerReference {
	control := true
	blockDeletion := true
	return metav1.OwnerReference{
		APIVersion:         apiVersion,
		Kind:               kind,
		Name:               name,
		UID:                uid,
		Controller:         &control,
		BlockOwnerDeletion: &blockDeletion,
	}
}

func mustGetCert(
	t *testing.T, ctx context.Context, cl client.Client, name string,
) certmanager.Certificate {
	t.Helper()
	var cert certmanager.Certificate
	require.Nil(t, cl.Get(ctx, types.NamespacedName{
		Namespace: ownershipTestNamespace,
		Name:      name,
	}, &cert))
	return cert
}

func certNotFound(ctx context.Context, t *testing.T, cl client.Client, name string) bool {
	t.Helper()
	var cert certmanager.Certificate
	err := cl.Get(ctx, types.NamespacedName{
		Namespace: ownershipTestNamespace,
		Name:      name,
	}, &cert)
	if apierrors.IsNotFound(err) {
		return true
	}
	require.Nil(t, err)
	return false
}

func listCerts(ctx context.Context, t *testing.T, cl client.Client) []certmanager.Certificate {
	t.Helper()
	var list certmanager.CertificateList
	require.Nil(t, cl.List(ctx, &list, client.InNamespace(ownershipTestNamespace)))
	return list.Items
}

func mustGetEndpoint(
	t *testing.T, ctx context.Context, cl client.Client, name string,
) externaldnsv1alpha1.DNSEndpoint {
	t.Helper()
	var ep externaldnsv1alpha1.DNSEndpoint
	require.Nil(t, cl.Get(ctx, types.NamespacedName{
		Namespace: ownershipTestNamespace,
		Name:      name,
	}, &ep))
	return ep
}

func endpointNotFound(ctx context.Context, t *testing.T, cl client.Client, name string) bool {
	t.Helper()
	var ep externaldnsv1alpha1.DNSEndpoint
	err := cl.Get(ctx, types.NamespacedName{
		Namespace: ownershipTestNamespace,
		Name:      name,
	}, &ep)
	if apierrors.IsNotFound(err) {
		return true
	}
	require.Nil(t, err)
	return false
}

func listEndpoints(
	ctx context.Context, t *testing.T, cl client.Client,
) []externaldnsv1alpha1.DNSEndpoint {
	t.Helper()
	var list externaldnsv1alpha1.DNSEndpointList
	require.Nil(t, cl.List(ctx, &list, client.InNamespace(ownershipTestNamespace)))
	return list.Items
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func repeat(s string, n int) string {
	result := make([]byte, n)
	for i := range result {
		result[i] = s[0]
	}
	return string(result)
}

func indexOf(items []string, target string) int {
	for i, item := range items {
		if item == target {
			return i
		}
	}
	return -1
}

// opTrackingClient records creates and deletes.
type opTrackingClient struct {
	client.Client
	ops *[]string
}

func (c *opTrackingClient) Create(
	ctx context.Context, obj client.Object, opts ...client.CreateOption,
) error {
	*c.ops = append(*c.ops, "create:"+obj.GetName())
	return c.Client.Create(ctx, obj, opts...)
}

func (c *opTrackingClient) Delete(
	ctx context.Context, obj client.Object, opts ...client.DeleteOption,
) error {
	*c.ops = append(*c.ops, "delete:"+obj.GetName())
	return c.Client.Delete(ctx, obj, opts...)
}

// createFailingClient fails creation of a single resource name.
type createFailingClient struct {
	client.Client
	groupResource schema.GroupResource
	failName      string
}

func (c *createFailingClient) Create(
	ctx context.Context, obj client.Object, opts ...client.CreateOption,
) error {
	if obj.GetName() == c.failName {
		return apierrors.NewAlreadyExists(c.groupResource, c.failName)
	}
	return c.Client.Create(ctx, obj, opts...)
}
