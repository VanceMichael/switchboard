package switchboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchesNoIngressClass(t *testing.T) {
	selector := NewSelector(nil, nil)
	assert.True(t, selector.Matches(map[string]string{}))
	assert.True(t, selector.Matches(map[string]string{
		"kubernetes.io/ingress.class": "test",
	}))
	assert.False(t, selector.Matches(map[string]string{
		"switchboard.borchero.com/ignore": "true",
	}))
	assert.False(t, selector.Matches(map[string]string{
		"kubernetes.io/ingress.class":     "test",
		"switchboard.borchero.com/ignore": "true",
	}))
	assert.False(t, selector.Matches(map[string]string{
		"switchboard.borchero.com/ignore": "all",
	}))
	assert.False(t, selector.Matches(map[string]string{
		"kubernetes.io/ingress.class":     "test",
		"switchboard.borchero.com/ignore": "all",
	}))
}

func TestMatchesIngressClass(t *testing.T) {
	cls := "ingress"
	selector := NewSelector(&cls, nil)
	assert.False(t, selector.Matches(map[string]string{}))
	assert.False(t, selector.Matches(map[string]string{
		"kubernetes.io/ingress.class": "test",
	}))
	assert.True(t, selector.Matches(map[string]string{
		"kubernetes.io/ingress.class": "ingress",
	}))
	assert.False(t, selector.Matches(map[string]string{
		"switchboard.borchero.com/ignore": "true",
	}))
	assert.False(t, selector.Matches(map[string]string{
		"kubernetes.io/ingress.class":     "test",
		"switchboard.borchero.com/ignore": "true",
	}))
	assert.False(t, selector.Matches(map[string]string{
		"kubernetes.io/ingress.class":     "ingress",
		"switchboard.borchero.com/ignore": "true",
	}))
	assert.False(t, selector.Matches(map[string]string{
		"switchboard.borchero.com/ignore": "all",
	}))
	assert.False(t, selector.Matches(map[string]string{
		"kubernetes.io/ingress.class":     "test",
		"switchboard.borchero.com/ignore": "all",
	}))
	assert.False(t, selector.Matches(map[string]string{
		"kubernetes.io/ingress.class":     "ingress",
		"switchboard.borchero.com/ignore": "all",
	}))
}

func TestMatchesIntegration(t *testing.T) {
	cls := "ingress"
	selector := NewSelector(&cls, nil)

	// Ignore all
	assert.False(t, selector.MatchesIntegration(map[string]string{
		"switchboard.borchero.com/ignore": "true",
	}, "external-dns"))
	assert.False(t, selector.MatchesIntegration(map[string]string{
		"switchboard.borchero.com/ignore": "all",
	}, "external-dns"))

	// Ignore only one
	assert.False(t, selector.MatchesIntegration(map[string]string{
		"switchboard.borchero.com/ignore": "external-dns",
	}, "external-dns"))
	assert.True(t, selector.MatchesIntegration(map[string]string{
		"switchboard.borchero.com/ignore": "cert-manager",
	}, "external-dns"))

	// Ignore multiple
	assert.False(t, selector.MatchesIntegration(map[string]string{
		"switchboard.borchero.com/ignore": "external-dns,cert-manager",
	}, "external-dns"))
	assert.False(t, selector.MatchesIntegration(map[string]string{
		"switchboard.borchero.com/ignore": "external-dns,cert-manager",
	}, "cert-manager"))
	assert.True(t, selector.MatchesIntegration(map[string]string{
		"switchboard.borchero.com/ignore": "external-dns,cert-manager",
	}, "unknown"))

	// Ignore with space in between
	assert.False(t, selector.MatchesIntegration(map[string]string{
		"switchboard.borchero.com/ignore": "external-dns, cert-manager",
	}, "external-dns"))
	assert.False(t, selector.MatchesIntegration(map[string]string{
		"switchboard.borchero.com/ignore": "external-dns, cert-manager",
	}, "cert-manager"))
	assert.True(t, selector.MatchesIntegration(map[string]string{
		"switchboard.borchero.com/ignore": "external-dns, cert-manager",
	}, "unknown"))
}

func TestMatchesNamespaceWithoutSelector(t *testing.T) {
	selector := NewSelector(nil, nil)
	assert.False(t, selector.RestrictsNamespaces())
	assert.True(t, selector.MatchesNamespace(map[string]string{}))
	assert.True(t, selector.MatchesNamespace(map[string]string{
		"switchboard.borchero.com/ingress": "enabled",
	}))
}

func TestMatchesNamespaceWithMatchLabels(t *testing.T) {
	labelSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{"switchboard.borchero.com/ingress": "enabled"},
	})
	assert.Nil(t, err)
	selector := NewSelector(nil, labelSelector)
	assert.True(t, selector.RestrictsNamespaces())

	assert.False(t, selector.MatchesNamespace(map[string]string{}))
	assert.False(t, selector.MatchesNamespace(map[string]string{
		"switchboard.borchero.com/ingress": "disabled",
	}))
	assert.True(t, selector.MatchesNamespace(map[string]string{
		"switchboard.borchero.com/ingress": "enabled",
	}))
	// Additional labels must not affect matching
	assert.True(t, selector.MatchesNamespace(map[string]string{
		"switchboard.borchero.com/ingress": "enabled",
		"other":                            "label",
	}))
}

func TestMatchesNamespaceWithMatchExpressions(t *testing.T) {
	labelSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      "environment",
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{"prod", "staging"},
		}},
	})
	assert.Nil(t, err)
	selector := NewSelector(nil, labelSelector)

	assert.False(t, selector.MatchesNamespace(map[string]string{}))
	assert.False(t, selector.MatchesNamespace(map[string]string{"environment": "dev"}))
	assert.True(t, selector.MatchesNamespace(map[string]string{"environment": "prod"}))
	assert.True(t, selector.MatchesNamespace(map[string]string{"environment": "staging"}))
}

func TestInvalidNamespaceSelector(t *testing.T) {
	_, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      "environment",
			Operator: "Bogus",
		}},
	})
	assert.NotNil(t, err)
}
