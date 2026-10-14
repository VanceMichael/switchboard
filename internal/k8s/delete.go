package k8s

import (
	"context"
	"fmt"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteIfFound deletes the given resource from the cluster and returns an error only if the
// deletion fails. If the resource does not exist, no error will be returned.
func DeleteIfFound(ctx context.Context, client client.Client, resource client.Object) error {
	if err := client.Delete(ctx, resource); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete existing resource: %w", err)
	}
	return nil
}

// DeleteIfOwned deletes the given resource only if it exists and is controlled by the provided
// owner. If the resource does not exist or is owned by someone else (e.g. a same-named resource
// created externally), it is left untouched and no error is returned.
func DeleteIfOwned(
	ctx context.Context, client client.Client, resource client.Object, owner metav1.Object,
) error {
	key := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}
	if err := client.Get(ctx, key, resource); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to look up existing resource: %w", err)
	}
	if !metav1.IsControlledBy(resource, owner) {
		// A same-named resource exists that is not owned by us: we must never touch it.
		return nil
	}
	if err := client.Delete(ctx, resource); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete owned resource: %w", err)
	}
	return nil
}
