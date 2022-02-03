/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package managedcluster

import (
	"context"
	"errors"
	"fmt"
	"github.com/Ealianis/caravel-mcm/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ManagedClusterReconciler reconciles a ManagedCluster object
type ManagedClusterReconciler struct {
	Client       client.Client
	CoreV1Client corev1.CoreV1Interface
	Scheme       *runtime.Scheme
}

//+kubebuilder:rbac:groups=cluster.aks-caravel.mcm,resources=managedclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.aks-caravel.mcm,resources=managedclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.aks-caravel.mcm,resources=managedclusters/finalizers,verbs=update

func NewController(kubeClient client.Client, coreV1Client corev1.CoreV1Interface, scheme *runtime.Scheme) *ManagedClusterReconciler {
	return &ManagedClusterReconciler{
		Client:       kubeClient,
		CoreV1Client: coreV1Client,
		Scheme:       scheme,
	}
}

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ManagedClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var mc v1alpha1.ManagedCluster
	err := r.Client.Get(ctx, req.NamespacedName, &mc)

	fmt.Println("Namespace:")
	fmt.Println(req.Namespace)
	fmt.Println("NamespacedName:")
	fmt.Println(req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}

	clientConfigs := mc.Spec.ManagedClusterClientConfigs
	if len(clientConfigs) < 1 {
		// Todo - Log & format proper error message.
		return ctrl.Result{}, errors.New("ManagedCluster has no config")
	}

	// Todo - ManagedClusterClientConfigs is an array. What is the selection logic?
	clientConfig := clientConfigs[0]
	secretRef := clientConfig.SecretRef

	// Todo-  What is the namespace for the secrets?
	secret, err := r.CoreV1Client.Secrets("cluster").
		Get(ctx, secretRef, metav1.GetOptions{})

	// ToDo - How to unwrap secret into type desired?
	sValue := secret.StringData

	//ToDo - Use secret to construct client.
	fmt.Println(sValue)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&v1alpha1.ManagedCluster{}).
		Complete(r)
}
