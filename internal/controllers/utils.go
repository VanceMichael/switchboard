package controllers

import (
	"context"
	"fmt"
	"log/slog"

	configv1 "github.com/borchero/switchboard/internal/config/v1"
	"github.com/borchero/switchboard/internal/ext"
	"github.com/borchero/switchboard/internal/integrations"
	"github.com/borchero/switchboard/internal/k8s"
	"github.com/borchero/switchboard/internal/switchboard"
	traefik "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func integrationsFromConfig(
	config configv1.Config, client client.Client,
) ([]integrations.Integration, error) {
	result := make([]integrations.Integration, 0)
	externalDNS := config.Integrations.ExternalDNS
	if config.Integrations.ExternalDNS != nil {
		if (externalDNS.TargetService == nil) == (len(externalDNS.TargetIPs) == 0) {
			return nil, fmt.Errorf(
				"exactly one of `targetService` and `targetIPs` must be set for external-dns",
			)
		}
		if externalDNS.TargetService != nil {
			result = append(result, integrations.NewExternalDNS(
				client, switchboard.NewServiceTarget(
					externalDNS.TargetService.Name,
					externalDNS.TargetService.Namespace,
				),
				externalDNS.TTL,
			))
		} else {
			result = append(result, integrations.NewExternalDNS(
				client, switchboard.NewStaticTarget(externalDNS.TargetIPs...),
				externalDNS.TTL,
			))
		}
	}

	certManager := config.Integrations.CertManager
	if certManager != nil {
		result = append(result, integrations.NewCertManager(client, certManager.Template))
	}
	return result, nil
}

func builderWithIntegrations(
	builder *builder.Builder,
	integrations []integrations.Integration,
	ctrlClient client.Client,
	logger *slog.Logger,
) *builder.Builder {
	// Reconcile whenever an owned resource of one of the integrations is modified
	for _, itg := range integrations {
		builder = builder.Owns(itg.OwnedResource())
	}

	// Watch for dependent resources if required
	for _, itg := range integrations {
		for _, watched := range itg.WatchedObjects() {
			var list traefik.IngressRouteList
			enqueue := watchEnqueueFunc(ctrlClient, logger, watched, &list,
				func(list *traefik.IngressRouteList) []client.Object {
					return ext.Map(list.Items, func(v traefik.IngressRoute) client.Object {
						return &v
					})
				},
			)
			builder = builder.Watches(
				watched,
				handler.EnqueueRequestsFromMapFunc(enqueue),
			)
		}
	}

	return builder
}

// watchEnqueueFunc builds the mapping function for a watched object. Endpoint slices are matched
// by namespace and the `kubernetes.io/service-name` label (their generated names are not known
// upfront and a service may own multiple slices). All other watched objects are matched by their
// exact name and namespace.
func watchEnqueueFunc[L client.ObjectList](
	ctrlClient client.Client,
	logger *slog.Logger,
	watched client.Object,
	list L,
	getItems func(L) []client.Object,
) func(ctx context.Context, obj client.Object) []reconcile.Request {
	if slice, ok := watched.(*discoveryv1.EndpointSlice); ok {
		labels := slice.GetLabels()
		namespace := slice.GetNamespace()
		return k8s.EnqueueMapFuncMatching(
			ctrlClient, logger,
			func(obj client.Object) bool {
				if obj.GetNamespace() != namespace {
					return false
				}
				for key, value := range labels {
					if obj.GetLabels()[key] != value {
						return false
					}
				}
				return true
			},
			list, getItems,
		)
	}
	return k8s.EnqueueMapFunc(ctrlClient, logger, watched, list, getItems)
}
