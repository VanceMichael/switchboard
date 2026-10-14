package switchboard

import (
	"strings"

	"k8s.io/apimachinery/pkg/labels"
)

const (
	ingressAnnotationKey = "kubernetes.io/ingress.class"
	ignoreAnnotationKey  = "switchboard.borchero.com/ignore"
)

// Selector allows to easily determine if a resource with a set of annotations should be processed.
type Selector struct {
	ingressClass *string
	// namespaceSelector optionally restricts processing to namespaces matching the selector. If
	// it is nil, all namespaces are in scope (cluster-wide operation).
	namespaceSelector labels.Selector
}

// NewSelector creates a new selector which selects resources with the
// `kubernetes.io/ingress.class` set to the specified value if it is not `nil`. If
// `namespaceSelector` is not nil, only resources in namespaces matching the selector are in
// scope.
func NewSelector(ingressClass *string, namespaceSelector labels.Selector) Selector {
	return Selector{ingressClass, namespaceSelector}
}

// RestrictsNamespaces returns whether the selector limits processing to a subset of namespaces.
func (s Selector) RestrictsNamespaces() bool {
	return s.namespaceSelector != nil
}

// MatchesNamespace determines whether the provided set of namespace labels matches the selector.
// If no namespace selector is configured, every namespace matches.
func (s Selector) MatchesNamespace(namespaceLabels map[string]string) bool {
	if s.namespaceSelector == nil {
		return true
	}
	return s.namespaceSelector.Matches(labels.Set(namespaceLabels))
}

// Matches determines whether the provided set of annotation match the selector. If the method
// returns `true`, the resource from which the annotations were obtained should be processed.
func (s Selector) Matches(annotations map[string]string) bool {
	// If the ignore annotation is set, selector never matches
	if ignore, ok := annotations[ignoreAnnotationKey]; ok {
		if ignore == "true" || ignore == "all" {
			return false
		}
	}

	// If the selector has an associated ingress class, the ingress class must match
	if s.ingressClass != nil {
		if ingressClass, ok := annotations[ingressAnnotationKey]; ok {
			return *s.ingressClass == ingressClass
		}
		// No ingress class present
		return false
	}

	// Otherwise, any ingress class is fine
	return true
}

// MatchesIntegration returns whether the provided set of annotations match the provided
// integration.
func (Selector) MatchesIntegration(annotations map[string]string, integration string) bool {
	if ignore, ok := annotations[ignoreAnnotationKey]; ok {
		if ignore == "true" || ignore == "all" {
			return false
		}
		// Iterate over list of values set for `ignore` annotation
		for _, ignored := range strings.Split(ignore, ",") {
			if strings.TrimSpace(ignored) == integration {
				return false
			}
		}
	}
	return true
}
