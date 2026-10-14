package k8s

import (
	"context"
	"testing"

	"github.com/borchero/switchboard/internal/k8tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestDeleteIfFound(t *testing.T) {
	// Setup
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	client := k8tests.NewClient(t, scheme)
	namespace, shutdown := k8tests.NewNamespace(ctx, t, client)
	defer shutdown()

	// Create a service
	service := k8tests.DummyService("my-service", namespace, 80)
	err := client.Create(ctx, &service)
	require.Nil(t, err)

	// Multiple deletes should not result in an error
	err = DeleteIfFound(ctx, client, &service)
	assert.Nil(t, err)

	err = DeleteIfFound(ctx, client, &service)
	assert.Nil(t, err)
}

func TestDeleteIfOwned(t *testing.T) {
	// Setup
	ctx := context.Background()
	scheme := k8tests.NewScheme()
	ctrlClient := k8tests.NewClient(t, scheme)
	namespace, shutdown := k8tests.NewNamespace(ctx, t, ctrlClient)
	defer shutdown()

	// The owner controls a first (owned) service while a second service of the same shape is
	// created externally.
	owner := k8tests.DummyService("owner", namespace, 80)
	err := ctrlClient.Create(ctx, &owner)
	require.Nil(t, err)

	owned := k8tests.DummyService("owned", namespace, 80)
	err = ctrl.SetControllerReference(&owner, &owned, scheme)
	require.Nil(t, err)
	err = ctrlClient.Create(ctx, &owned)
	require.Nil(t, err)

	foreign := k8tests.DummyService("foreign", namespace, 80)
	err = ctrlClient.Create(ctx, &foreign)
	require.Nil(t, err)

	// The owned resource is deleted while the foreign resource is left untouched
	ownedLookup := k8tests.DummyService("owned", namespace, 80)
	err = DeleteIfOwned(ctx, ctrlClient, &ownedLookup, &owner)
	assert.Nil(t, err)

	ownedAfterDelete := k8tests.DummyService("owned", namespace, 80)
	err = ctrlClient.Get(ctx, types.NamespacedName{Name: "owned", Namespace: namespace}, &ownedAfterDelete)
	assert.True(t, apierrors.IsNotFound(err))

	foreignLookup := k8tests.DummyService("foreign", namespace, 80)
	err = DeleteIfOwned(ctx, ctrlClient, &foreignLookup, &owner)
	assert.Nil(t, err)
	foreignActual := k8tests.DummyService("foreign", namespace, 80)
	err = ctrlClient.Get(ctx, types.NamespacedName{Name: "foreign", Namespace: namespace}, &foreignActual)
	assert.Nil(t, err)

	// A missing resource and repeated calls do not result in errors
	ownedLookup2 := k8tests.DummyService("owned", namespace, 80)
	err = DeleteIfOwned(ctx, ctrlClient, &ownedLookup2, &owner)
	assert.Nil(t, err)
	missing := k8tests.DummyService("missing", namespace, 80)
	err = DeleteIfOwned(ctx, ctrlClient, &missing, &owner)
	assert.Nil(t, err)
}
