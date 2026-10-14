package k8s

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/borchero/switchboard/internal/ext"
	"github.com/borchero/switchboard/internal/k8tests"
	certmanager "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	traefik "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestEnqueueMapFunc(t *testing.T) {
	// Setup
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	ctrlClient := k8tests.NewClient(t, scheme)
	namespace, shutdown := k8tests.NewNamespace(ctx, t, ctrlClient)
	defer shutdown()

	// Create a couple of services
	service1 := k8tests.DummyService("my-service-1", namespace, 80)
	err := ctrlClient.Create(ctx, &service1)
	require.Nil(t, err)
	service2 := k8tests.DummyService("my-service-2", namespace, 80)
	err = ctrlClient.Create(ctx, &service2)
	require.Nil(t, err)
	service3 := k8tests.DummyService("my-service-3", namespace, 80)
	err = ctrlClient.Create(ctx, &service3)
	require.Nil(t, err)

	// Create the enqueue function which is triggered by service 1
	var services v1.ServiceList
	enqueuer := EnqueueMapFunc(
		ctrlClient, slog.Default(), &service1, &services,
		func(list *v1.ServiceList) []client.Object {
			return ext.Map(list.Items, func(v v1.Service) client.Object { return &v })
		},
	)

	// Check whether enqueue only happens for service1
	assert.Greater(t, len(enqueuer(ctx, &service1)), 0)
	assert.Len(t, enqueuer(ctx, &service2), 0)
	assert.Len(t, enqueuer(ctx, &service3), 0)

	// Check whether distinct services are returned for enqueue
	names := []string{"my-service-1", "my-service-2", "my-service-3"}
	var found []string

	for _, obj := range enqueuer(ctx, &service1) {
		if obj.Namespace == namespace {
			found = append(found, obj.Name)
		}
	}
	assert.ElementsMatch(t, names, found)
}

func TestEnqueuePreferredOwnerFunc(t *testing.T) {
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	namespace := "preferred-owner"

	ingress := &traefik.IngressRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "my-ingress", Namespace: namespace},
	}
	other := &traefik.IngressRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "other-ingress", Namespace: namespace},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ingress, other).Build()

	newOwner := func() client.Object { return &traefik.IngressRoute{} }

	// Certificate resolver: preferred child names end with "-tls".
	certResolve := func(name string) (string, bool) {
		if !strings.HasSuffix(name, "-tls") {
			return "", false
		}
		return strings.TrimSuffix(name, "-tls"), true
	}
	certEnqueuer := EnqueuePreferredOwnerFunc(slog.Default(), cl, newOwner, certResolve)

	certEvent := func(name string) client.Object {
		return &certmanager.Certificate{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: namespace,
		}}
	}

	// Foreign certificate occupying the preferred name enqueues precisely the single route.
	assert.Equal(t, []reconcile.Request{{
		NamespacedName: types.NamespacedName{Name: "my-ingress", Namespace: namespace},
	}}, certEnqueuer(ctx, certEvent("my-ingress-tls")))

	// A preferred name without a matching IngressRoute enqueues nothing.
	assert.Empty(t, certEnqueuer(ctx, certEvent("missing-ingress-tls")))

	// Names not matching the preferred pattern enqueue nothing.
	assert.Empty(t, certEnqueuer(ctx, certEvent("plain-name")))

	// Other namespaces cannot enqueue the route.
	assert.Empty(t, certEnqueuer(ctx, &certmanager.Certificate{ObjectMeta: metav1.ObjectMeta{
		Name: "my-ingress-tls", Namespace: "other-namespace",
	}}))

	// DNSEndpoint resolver: preferred name is identical to the route name.
	dnsEnqueuer := EnqueuePreferredOwnerFunc(slog.Default(), cl, newOwner,
		func(name string) (string, bool) { return name, true },
	)
	assert.Equal(t, []reconcile.Request{{
		NamespacedName: types.NamespacedName{Name: "other-ingress", Namespace: namespace},
	}}, dnsEnqueuer(ctx, certEvent("other-ingress")))
	assert.Empty(t, dnsEnqueuer(ctx, certEvent("unknown-ingress")))
}
