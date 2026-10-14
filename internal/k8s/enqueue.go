package k8s

import (
	"context"
	"log/slog"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// EnqueueMapFunc may be used to watch changes of a particular target objects and trigger the
// reconciliation of resources of a particular type. Only events for the exact target object
// (matching kind, namespace, and name) cause resources to be enqueued. If `isManaged` is not
// nil, it is evaluated for every candidate resource and only resources for which it returns
// `true` are enqueued. The given logger is used to log errors in the background.
func EnqueueMapFunc[L client.ObjectList](
	ctrlClient client.Client,
	logger *slog.Logger,
	target client.Object,
	list L,
	getItems func(L) []client.Object,
	isManaged func(context.Context, client.Object) bool,
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
		requests := make([]reconcile.Request, 0, len(items))
		for _, item := range items {
			if isManaged != nil && !isManaged(ctx, item) {
				continue
			}
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(item),
			})
		}
		return requests
	}
}
