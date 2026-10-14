package controllers

import (
	"context"

	traefik "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// isManagedRoute determines whether the given object (expected to be an IngressRoute) is
// currently taken over by Switchboard: it must be located in a matching namespace (if a
// namespace selector is configured) and its annotations must match the ingress class / ignore
// rules. If namespace membership cannot be determined because the namespace lookup fails, the
// route is considered not managed: enqueueing nothing preserves existing derived resources and
// the regular reconcile loop converges once the API is available again.
func (r *IngressRouteReconciler) isManagedRoute(
	ctx context.Context, obj client.Object,
) bool {
	if r.selector.RestrictsNamespaces() {
		var namespace v1.Namespace
		if err := r.Get(
			ctx, client.ObjectKey{Name: obj.GetNamespace()}, &namespace,
		); err != nil {
			r.logger.Error("unable to query namespace while filtering routes to enqueue",
				"namespace", obj.GetNamespace(), "error", err,
			)
			return false
		}
		if !r.selector.MatchesNamespace(namespace.Labels) {
			return false
		}
	}
	return r.selector.Matches(obj.GetAnnotations())
}

// enqueueIngressRoutesInNamespace maps a namespace event to reconciliation requests for all
// (and only) the ingress routes located in that namespace. A failure to list the routes does
// not release any resources (no reconcile is triggered) and is logged: the event is retried
// implicitly through subsequent events and the informer resync.
func (r *IngressRouteReconciler) enqueueIngressRoutesInNamespace(
	ctx context.Context, obj client.Object,
) []reconcile.Request {
	if _, ok := obj.(*v1.Namespace); !ok {
		return nil
	}
	var routes traefik.IngressRouteList
	if err := r.List(ctx, &routes, client.InNamespace(obj.GetName())); err != nil {
		r.logger.Error("failed to list ingress routes in changed namespace",
			"namespace", obj.GetName(), "error", err,
		)
		return nil
	}
	requests := make([]reconcile.Request, 0, len(routes.Items))
	for i := range routes.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&routes.Items[i]),
		})
	}
	return requests
}
