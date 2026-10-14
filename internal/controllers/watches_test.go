package controllers

import (
	"context"
	"log/slog"
	"testing"

	"github.com/borchero/switchboard/internal/k8tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	traefik "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestEnqueueIngressRoutesInNamespace(t *testing.T) {
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	ctrlClient := k8tests.NewClient(t, scheme)

	namespaceA, shutdownA := k8tests.NewNamespace(ctx, t, ctrlClient)
	defer shutdownA()
	namespaceB, shutdownB := k8tests.NewNamespace(ctx, t, ctrlClient)
	defer shutdownB()
	setNamespaceLabels(ctx, t, ctrlClient, namespaceA,
		map[string]string{namespaceLabelKey: "enabled"},
	)

	routeA := traefik.IngressRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "ingress-a", Namespace: namespaceA},
		Spec: traefik.IngressRouteSpec{
			Routes: []traefik.Route{{
				Kind: "Rule", Match: "Host(`a.example.com`)",
			}},
		},
	}
	require.Nil(t, ctrlClient.Create(ctx, &routeA))
	routeB := traefik.IngressRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "ingress-b", Namespace: namespaceB},
		Spec: traefik.IngressRouteSpec{
			Routes: []traefik.Route{{
				Kind: "Rule", Match: "Host(`b.example.com`)",
			}},
		},
	}
	require.Nil(t, ctrlClient.Create(ctx, &routeB))

	reconciler, err := NewIngressRouteReconciler(ctrlClient, slog.Default(), namespaceSelectorConfig())
	require.Nil(t, err)

	// A namespace event enqueues all (and only) ingress routes of that namespace.
	var namespaceObjectA v1.Namespace
	require.Nil(t, ctrlClient.Get(ctx, types.NamespacedName{Name: namespaceA}, &namespaceObjectA))
	requestsA := reconciler.enqueueIngressRoutesInNamespace(ctx, &namespaceObjectA)
	require.Len(t, requestsA, 1)
	assert.Equal(t, "ingress-a", requestsA[0].Name)
	assert.Equal(t, namespaceA, requestsA[0].Namespace)

	var namespaceObjectB v1.Namespace
	require.Nil(t, ctrlClient.Get(ctx, types.NamespacedName{Name: namespaceB}, &namespaceObjectB))
	requestsB := reconciler.enqueueIngressRoutesInNamespace(ctx, &namespaceObjectB)
	require.Len(t, requestsB, 1)
	assert.Equal(t, "ingress-b", requestsB[0].Name)

	// Objects other than namespaces are ignored.
	assert.Nil(t, reconciler.enqueueIngressRoutesInNamespace(ctx, &v1.Service{}))

	// Only routes in matching namespaces are currently managed.
	assert.True(t, reconciler.isManagedRoute(ctx, &routeA))
	assert.False(t, reconciler.isManagedRoute(ctx, &routeB))
}
