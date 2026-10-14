package k8s

import (
	"context"
	"log/slog"
	"testing"

	"github.com/borchero/switchboard/internal/ext"
	"github.com/borchero/switchboard/internal/k8tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestEnqueueMapFuncMatchingEndpointSlice(t *testing.T) {
	// Uses a fake client (no live cluster required): endpoint slices are matched by namespace
	// and the `kubernetes.io/service-name` label, mirroring how the ingress route controller
	// reconciles upon slice creation, update (address/readiness), and deletion.
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	const namespace = "ns"
	const serviceName = "headless"

	configMap1 := v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "ingress-1", Namespace: namespace}}
	configMap2 := v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "ingress-2", Namespace: "other-ns"}}
	ctrlClient := fake.NewClientBuilder().
		WithScheme(scheme).WithObjects(&configMap1, &configMap2).Build()

	watched := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Labels:    map[string]string{discoveryv1.LabelServiceName: serviceName},
		},
	}
	var configMaps v1.ConfigMapList
	enqueuer := EnqueueMapFuncMatching(
		ctrlClient, slog.Default(),
		func(obj client.Object) bool {
			if obj.GetNamespace() != watched.GetNamespace() {
				return false
			}
			for key, value := range watched.GetLabels() {
				if obj.GetLabels()[key] != value {
					return false
				}
			}
			return true
		},
		&configMaps,
		func(list *v1.ConfigMapList) []client.Object {
			return ext.Map(list.Items, func(v v1.ConfigMap) client.Object { return &v })
		},
	)

	// Slice of the target service (regardless of its generated name) triggers reconciliation.
	matchingSlice := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "headless-abc123",
			Namespace: namespace,
			Labels:    map[string]string{discoveryv1.LabelServiceName: serviceName},
		},
	}
	requests := enqueuer(ctx, &matchingSlice)
	assert.Len(t, requests, 2)

	// A readiness/address update event carries the same labels and enqueues as well.
	matchingSlice.Endpoints = []discoveryv1.Endpoint{{
		Addresses:  []string{"10.0.0.1"},
		Conditions: discoveryv1.EndpointConditions{Ready: boolPtr(false)},
	}}
	assert.Len(t, enqueuer(ctx, &matchingSlice), 2)

	// A slice associated with another service does not trigger reconciliation.
	otherSlice := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-def456",
			Namespace: namespace,
			Labels:    map[string]string{discoveryv1.LabelServiceName: "other-service"},
		},
	}
	assert.Len(t, enqueuer(ctx, &otherSlice), 0)

	// A slice of the target service name in another namespace does not trigger either.
	foreignSlice := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "headless-xyz789",
			Namespace: "other-ns",
			Labels:    map[string]string{discoveryv1.LabelServiceName: serviceName},
		},
	}
	assert.Len(t, enqueuer(ctx, &foreignSlice), 0)
}

func boolPtr(value bool) *bool {
	return &value
}
