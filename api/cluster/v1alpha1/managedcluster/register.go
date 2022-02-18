package managedcluster

import (
	cluster "github.com/Ealianis/caravel-mcm/api/cluster"
	"github.com/Ealianis/caravel-mcm/api/cluster/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// Kind is normally the CamelCased singular type. The resource manifest uses this.
	Kind string = "ManagedCluster"
	// GroupVersion is the version.
	GroupVersion string = v1alpha1.Version
	// Plural is the plural name used in /apis/<group>/<version>/<plural>
	Plural string = "ManagedClusters"
	// Singular is used as an alias on kubectl for display.
	Singular string = "ManagedCluster"
)

var (
	// SchemeGroupVersion is the group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{
		Group:   cluster.GroupName,
		Version: GroupVersion,
	}
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// addKnownTypes adds the set of types defined in this package to the supplied scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ManagedCluster{},
		&ManagedClusterList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)

	return nil
}
