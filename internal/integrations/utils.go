package integrations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"strings"

	"dario.cat/mergo"
	"github.com/borchero/switchboard/internal/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// resourceNameMaxLength is the maximum length of a DNS-1123 subdomain name as used by the
// resources managed by integrations.
const resourceNameMaxLength = 253

func reconcileMetadata(
	owner metav1.Object, target metav1.Object, scheme *runtime.Scheme, sources ...metav1.Object,
) error {
	// Reconcile labels
	labels := defaultEmpty(target.GetLabels())
	labels[managedByLabelKey] = "switchboard"
	for _, source := range sources {
		if err := mergo.MergeWithOverwrite(&labels, source.GetLabels()); err != nil {
			return fmt.Errorf("failed to update labels: %s", err)
		}
	}
	target.SetLabels(labels)

	// Reconcile annotations
	annotations := defaultEmpty(target.GetAnnotations())
	if ingressClass, ok := owner.GetAnnotations()[ingressAnnotationKey]; ok {
		annotations[ingressAnnotationKey] = ingressClass
	} else {
		delete(annotations, ingressAnnotationKey)
	}
	for _, source := range sources {
		if err := mergo.MergeWithOverwrite(&annotations, source.GetAnnotations()); err != nil {
			return fmt.Errorf("failed to update annotations: %s", err)
		}
	}
	target.SetAnnotations(annotations)

	// Set controller reference
	if err := ctrl.SetControllerReference(owner, target, scheme); err != nil {
		return err
	}
	return nil
}

func defaultEmpty(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	return m
}

//-------------------------------------------------------------------------------------------------
// OWNERSHIP
//-------------------------------------------------------------------------------------------------

// childResources contains the typed accessors that are required to generically reconcile the
// single child resource of an integration (e.g. a Certificate or a DNSEndpoint).
type childResources struct {
	client    client.Client
	scheme    *runtime.Scheme
	newObject func(namespace, name string) client.Object
	newList   func() client.ObjectList
	listItems func(client.ObjectList) []client.Object
}

// reconcile ensures that the child resource derived from the given owner matches the desired
// state. Resource identity is determined solely by the controller ownerReference's kind, name and
// UID - never by the resource name alone.
//
// If want is false, all children controlled by the current owner UID are deleted while foreign
// resources sharing the preferred or alternative name are preserved.
//
// If want is true, reconcile uses the preferred name when it is free or owned by the current
// owner UID. If the preferred name is occupied by a foreign resource (no controller, another
// controller, or a stale UID), the foreign resource is preserved untouched and a deterministic
// alternative name is used instead. When the preferred name becomes available again, the
// preferred child is created or repaired first and only then the UID-owned alternative child is
// removed - a failed preferred upsert never invalidates the still-serving alternative.
func (r childResources) reconcile(
	ctx context.Context,
	owner metav1.Object,
	preferred string,
	want bool,
	mutate func(client.Object) error,
) error {
	kind, err := ownerKind(r.scheme, owner)
	if err != nil {
		return err
	}

	namespace := owner.GetNamespace()
	ownerName := owner.GetName()
	ownerUID := owner.GetUID()

	// Find all children currently controlled by this exact owner identity (kind/name/UID).
	owned, err := r.ownedBy(ctx, namespace, kind, ownerName, ownerUID)
	if err != nil {
		return fmt.Errorf("failed to list owned resources: %w", err)
	}

	if !want {
		// Cleanup: only remove children controlled by the current UID. Foreign resources that
		// happen to share the preferred or alternative name are preserved.
		for _, child := range owned {
			if err := k8s.DeleteIfFound(ctx, r.client, child); err != nil {
				return fmt.Errorf("failed to delete owned resource %q: %w", child.GetName(), err)
			}
		}
		return nil
	}

	alternative := AlternativeName(preferred, ownerUID)

	preferredObj, preferredFound, err := r.find(ctx, namespace, preferred)
	if err != nil {
		return fmt.Errorf("failed to look up preferred resource %q: %w", preferred, err)
	}
	alternativeObj, alternativeFound, err := r.find(ctx, namespace, alternative)
	if err != nil {
		return fmt.Errorf("failed to look up alternative resource %q: %w", alternative, err)
	}

	if preferredFound && !controlledBy(preferredObj, kind, ownerName, ownerUID) {
		// The preferred name is occupied by a foreign resource (without controller, controlled
		// by another controller, or controlled by a stale owner UID). It must be preserved
		// untouched and we use the stable alternative name instead.
		if alternativeFound && !controlledBy(alternativeObj, kind, ownerName, ownerUID) {
			return fmt.Errorf(
				"preferred resource %q and alternative resource %q are both controlled by "+
					"another owner",
				preferred, alternative,
			)
		}
		target := r.newObject(namespace, alternative)
		if alternativeFound {
			target = alternativeObj
		}
		if err := r.upsert(ctx, target, mutate); err != nil {
			return fmt.Errorf("failed to upsert alternative resource %q: %w", alternative, err)
		}
		return r.deleteStrays(ctx, owned, alternative)
	}

	// The preferred name is free or already ours: create or repair the preferred child first.
	target := r.newObject(namespace, preferred)
	if preferredFound {
		target = preferredObj
	}
	if err := r.upsert(ctx, target, mutate); err != nil {
		// Do not delete the alternative child: it may still serve traffic. The returned error
		// triggers a retry which reattempts moving to the preferred name without a gap.
		return fmt.Errorf("failed to upsert preferred resource %q: %w", preferred, err)
	}

	// The preferred child is healthy: only now remove any UID-owned alternative child.
	return r.deleteStrays(ctx, owned, preferred)
}

func (r childResources) upsert(
	ctx context.Context, obj client.Object, mutate func(client.Object) error,
) error {
	if obj.GetResourceVersion() == "" {
		if err := mutate(obj); err != nil {
			return err
		}
		return r.client.Create(ctx, obj)
	}

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	if err := mutate(obj); err != nil {
		return err
	}
	return r.client.Patch(ctx, obj, patch)
}

func (r childResources) find(
	ctx context.Context, namespace, name string,
) (client.Object, bool, error) {
	obj := r.newObject(namespace, name)
	if err := r.client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}
		return nil, false, nil
	}
	return obj, true, nil
}

func (r childResources) ownedBy(
	ctx context.Context, namespace, kind, name string, uid types.UID,
) ([]client.Object, error) {
	list := r.newList()
	if err := r.client.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	items := r.listItems(list)
	result := make([]client.Object, 0, len(items))
	for _, item := range items {
		if controlledBy(item, kind, name, uid) {
			result = append(result, item)
		}
	}
	return result, nil
}

func (r childResources) deleteStrays(
	ctx context.Context, owned []client.Object, keep string,
) error {
	for _, child := range owned {
		if child.GetName() == keep {
			continue
		}
		if err := k8s.DeleteIfFound(ctx, r.client, child); err != nil {
			return fmt.Errorf("failed to delete stale owned resource %q: %w", child.GetName(), err)
		}
	}
	return nil
}

// ownerKind resolves the kind of the given owner object using the provided scheme - mirroring
// how controllerutil.SetControllerReference derives the ownerReference kind.
func ownerKind(scheme *runtime.Scheme, owner metav1.Object) (string, error) {
	ro, ok := owner.(runtime.Object)
	if !ok {
		return "", fmt.Errorf("owner %T does not implement runtime.Object", owner)
	}
	gvks, _, err := scheme.ObjectKinds(ro)
	if err != nil {
		return "", fmt.Errorf("failed to resolve owner kind: %w", err)
	}
	return gvks[0].Kind, nil
}

// controlledBy reports whether the given object has a controller ownerReference matching the
// provided kind, name and UID. Objects without a controller reference or with a controller
// reference of another kind, name or UID (e.g. a same-named owner that was recreated with a new
// UID) are not considered controlled.
func controlledBy(obj client.Object, kind, name string, uid types.UID) bool {
	ref := metav1.GetControllerOf(obj)
	if ref == nil {
		return false
	}
	return ref.Kind == kind && ref.Name == name && ref.UID == uid
}

// AlternativeName returns a deterministic, DNS-1123-legal child name derived from the preferred
// name and the owner's UID. The same owner always maps to the same alternative across retries and
// manager restarts while distinct UIDs (e.g. a same-named IngressRoute recreated after deletion)
// map to distinct alternatives.
func AlternativeName(preferred string, uid types.UID) string {
	sum := sha256.Sum256([]byte(uid))
	suffix := hex.EncodeToString(sum[:8]) // 16 lowercase hexadecimal characters

	maxBody := resourceNameMaxLength - len(suffix) - 1
	body := preferred
	if len(body) > maxBody {
		body = body[:maxBody]
	}
	// The suffix starts with a hexadecimal character, so we only need to ensure the body does
	// not end with a dash to keep the composed name legal.
	body = strings.TrimRight(body, "-")
	return body + "-" + suffix
}

// cloneAnnotations returns a shallow copy of the provided annotations map.
func cloneAnnotations(annotations map[string]string) map[string]string {
	if len(annotations) == 0 {
		return nil
	}
	return maps.Clone(annotations)
}
