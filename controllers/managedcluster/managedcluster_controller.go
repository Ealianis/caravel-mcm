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
	"fmt"
	"github.com/Ealianis/caravel-mcm/api/v1alpha1"
	"github.com/Ealianis/caravel-mcm/api/v1alpha1/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ManagedClusterReconciler reconciles a ManagedCluster object
type ManagedClusterReconciler struct {
	Client       client.Client
	CoreV1Client corev1.CoreV1Interface
	Scheme       *runtime.Scheme
	clusterMap   map[string]v1alpha1.ManagedCluster
	fleetId      string
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
	log := log.FromContext(ctx)

	var mc v1alpha1.ManagedCluster
	err := r.Client.Get(ctx, req.NamespacedName, &mc)

	//Fleet Logic
	exists, err := r.checkIfFleetExists(mc, req.NamespacedName.Name)
	if err != nil || exists {
		return ctrl.Result{}, err
	}

	// TODO: After creating more fields for lease status, add here.
	newLease := v1alpha1.MemberClusterLease{
		TypeMeta:   metav1.TypeMeta{Kind: "MemberClusterLease", APIVersion: "v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       v1alpha1.MemberClusterLeaseSpec{FleetID: r.fleetId},
	}
	mc.Lease = newLease
	r.clusterMap[req.NamespacedName.Name] = mc

	//Creating Clients
	clientConfigs := mc.Spec.ManagedClusterClientConfigs
	if len(clientConfigs) < 1 {
		log.Error(errors.WrongUrlOrCredentials(), "ManagedCluster has no config")
		return ctrl.Result{}, errors.WrongUrlOrCredentials()
	}

	var secretRef string
	secret, err := r.CoreV1Client.Secrets("member-cluster-kubeconfigs").
		Get(ctx, secretRef, metav1.GetOptions{})

	//Assuming that there could be more than 1 configs in the array
	var url string
	for _, value := range clientConfigs {
		if value.SecretRef == secret.Name {
			url = value.URL
			secretRef = value.SecretRef
			break
		}
	}
	if url == "" {
		return ctrl.Result{}, errors.WrongUrlOrCredentials()
	}
	//secret was SecretRef
	mcKubeconfig, err := r.GetMemberClusterKubeConfig(secretRef, "member-cluster-kubeconfigs")

	restConfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
		return clientcmd.Load([]byte(mcKubeconfig))
	})
	if err != nil {
		log.Error(err, "unable to parse kubeconfig")
		return ctrl.Result{}, err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return ctrl.Result{}, errors.ClientNotCreated()
	}

	kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})

	return ctrl.Result{}, nil
}

func (r *ManagedClusterReconciler) GetMemberClusterKubeConfig(secretName, secretNamespace string) (string, error) {
	var secret v1.Secret
	namespacedName := types.NamespacedName{Namespace: secretNamespace, Name: secretName}
	if err := r.Client.Get(context.Background(), namespacedName, &secret); err != nil {
		return "", err
	}

	kubeconfig, ok := secret.Data["kubeconfig"]
	if !ok || len(kubeconfig) == 0 {
		return "", fmt.Errorf("kubeconfig not found in secret %s", namespacedName)
	}

	return string(kubeconfig), nil
}

func (r *ManagedClusterReconciler) checkIfFleetExists(memberCluster v1alpha1.ManagedCluster, ns string) (bool, error) {
	if value, exists := r.clusterMap[ns]; !exists {
		fleetId := memberCluster.Lease.Spec.FleetID
		if fleetId != "" {
			return true, errors.LeaseAlreadyExists(value.Lease.Spec.FleetID)
		}
	}
	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.clusterMap = make(map[string]v1alpha1.ManagedCluster)
	r.fleetId = string(uuid.NewUUID())
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&v1alpha1.ManagedCluster{}).
		Complete(r)
}
