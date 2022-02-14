package memberclusterlease

import (
	"fmt"
	"github.com/Ealianis/caravel-mcm/api/cluster"
	"github.com/Ealianis/caravel-mcm/types"
	"reflect"
	"time"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

// CreateCustomResourceDefinition creates the CRD and add it into Kubernetes. If there is error,
// it will do some clean up.
func CreateCustomResourceDefinition(clientSet apiextensionsclientset.Interface, crdName string) (*apiextensionsv1beta1.CustomResourceDefinition, error) {
	crd := &apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: crdName,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   cluster.GroupName,
			Version: SchemeGroupVersion.Version,
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural:     Plural,
				Singular:   Singular,
				Kind:       reflect.TypeOf(MemberClusterLease{}).Name(),
				ShortNames: []string{ShortName},
			},
			Validation: &apiextensionsv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &apiextensionsv1beta1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]apiextensionsv1beta1.JSONSchemaProps{
						"spec": {
							Type: "object",
							Properties: map[string]apiextensionsv1beta1.JSONSchemaProps{
								"fleetId":            {Type: "string", Format: "string"},
								"lastJoinTime":       {Type: "string", Format: "date-time"},
								"lastLeaseRenewTime": {Type: "string", Format: "date-time"},
							},
						},
					},
					Required: []string{"fleetId", "lastJoinTime", "lastLeaseRenewTime"},
				},
			},
		},
	}
	ctx := types.GetCtx()
	_, err := clientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
	if err == nil {
		fmt.Println("CRD MemberClusterLease is created")
	} else if apierrors.IsAlreadyExists(err) {
		fmt.Println("CRD MemberClusterLease already exists")
	} else {
		fmt.Printf("Fail to create CRD MemberClusterLease: %+v\n", err)

		return nil, err
	}

	// Wait for CRD creation.
	err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		crd, err = clientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Get(ctx, CRDName, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Fail to wait for CRD MemberClusterLease creation: %+v\n", err)

			return false, err
		}
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1beta1.Established:
				if cond.Status == apiextensionsv1beta1.ConditionTrue {
					return true, err
				}
			case apiextensionsv1beta1.NamesAccepted:
				if cond.Status == apiextensionsv1beta1.ConditionFalse {
					fmt.Printf("Name conflict while wait for CRD MemberClusterLease creation: %s, %+v\n", cond.Reason, err)
				}
			}
		}

		return false, err
	})

	// If there is an error, delete the object to keep it clean.
	if err != nil {
		fmt.Println("Try to cleanup")
		deleteErr := clientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(ctx, CRDName, metav1.DeleteOptions{})
		if deleteErr != nil {
			fmt.Printf("Fail to delete CRD MemberClusterLease: %+v\n", deleteErr)

			return nil, errors.NewAggregate([]error{err, deleteErr})
		}

		return nil, err
	}

	return crd, nil
}
