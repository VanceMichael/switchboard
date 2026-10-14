package k8s

import (
	"context"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// EnqueueMapFunc may be used to watch changes of a particular target objects and trigger the
// reconciliation of all resources of a particular type. The given logger is used to log errors in
// the background.
func EnqueueMapFunc[L client.ObjectList](
	ctrlClient client.Client,
	logger *slog.Logger,
	target client.Object,
	list L,
	getItems func(L) []client.Object,
) func(context.Context, client.Object) []reconcile.Request {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		// Check whether we need to enqueue any objects
		if target.GetObjectKind() != obj.GetObjectKind() ||
			target.GetNamespace() != obj.GetNamespace() ||
			target.GetName() != obj.GetName() {
			return nil
		}

		// If our filter matches, we want to fetch all items of the specified type...
		if err := ctrlClient.List(ctx, list); err != nil {
			logger.Error("failed to list resources upon object change", "error", err)
			return nil
		}

		// ...and map them to reconciliation requests
		items := getItems(list)
		requests := make([]reconcile.Request, len(items))
		for i, item := range items {
			requests[i].Name = item.GetName()
			requests[i].Namespace = item.GetNamespace()
		}
		return requests
	}
}

// EnqueuePreferredOwnerFunc returns a map function for child resource events which enqueues
// exactly the single owner whose preferred child name matches the event object's name. It is used
// to watch child resources (including resources not owned by this controller) so that updates or
// deletions of a foreign object occupying an owner's preferred name trigger the reconciliation
// of precisely that owner. Events for resources unrelated to any existing owner - including
// arbitrary foreign resources - produce no requests and never cause a full reconciliation.
func EnqueuePreferredOwnerFunc(
	logger *slog.Logger,
	ctrlClient client.Client,
	newOwner func() client.Object,
	resolve func(childName string) (ownerName string, ok bool),
) func(context.Context, client.Object) []reconcile.Request {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		ownerName, ok := resolve(obj.GetName())
		if !ok {
			return nil
		}

		key := types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      ownerName,
		}
		owner := newOwner()
		if err := ctrlClient.Get(ctx, key, owner); err != nil {
			if !apierrors.IsNotFound(err) {
				logger.Error("failed to look up owner for child resource event",
					"resource", obj.GetName(), "error", err)
			}
			return nil
		}
		return []reconcile.Request{{NamespacedName: key}}
	}
}
