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
	mct "github.com/Ealianis/caravel-mcm/api/cluster/v1alpha1/managedcluster"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
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
	clusterMap   map[string]mct.ManagedCluster
	fleetId      string
}

const (
	managedClusterKubeConfigSecretNamespace = "managed-cluster-kubeconfigs"
	kubeConfigDataKey                       = "kubeconfig"
)

var (
	errorInvalidKubeConfig                     = errors.New("the ManagedCluster's KubeConfig is invalid")
	errorUnableToFindManagedClusterResource    = errors.New("the ManagedCluster resource could not be retrieved")
	errorManagedClusterJoinedToDifferentFleet  = errors.New("the ManagedCluster belongs to another fleet")
	errorMissingManagedClusterClientConfig     = errors.New("the ManagedCluster does not have a valid client configuration")
	errorMissingManagedClusterKubeConfigSecret = errors.New("the ManagedCluster is missing a KubeConfig secret")
)

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

	var mc mct.ManagedCluster
	err := r.Client.Get(ctx, req.NamespacedName, &mc)

	if err != nil {
		log.Error(errorUnableToFindManagedClusterResource, "", nil)
		return ctrl.Result{}, errorUnableToFindManagedClusterResource
	}

	// Any ManagedCluster that does not have a valid KubeConfig, and thus a KubeClient can not be constructed for,
	// should have its reconciliation stopped and logged.
	managedClusterKubeClient, err := r.ConstructManagedClusterKubeClientFromClientConfig(mc.Spec.ManagedClusterClientConfigs)

	if err != nil {
		log.Error(err, "", nil)
		return ctrl.Result{}, err
	}

	// use kubeclient to do work.
	if err := r.ReconcileManagedClusterFleetStatus(managedClusterKubeClient); err != nil {
		// Does a CRD "FOO" Exist?
		// instlal CRD on spoke
		// CREATE CRD
		//managedClusterKubeClient.log.Error(err, "", nil)
	}

	nodeList, err := managedClusterKubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	for _, node := range nodeList.Items {
		mc.Status.Conditions = node.Status.Conditions
		if mc.Status.Capacity == nil {
			mc.Status.Capacity = map[v1.ResourceName]resource.Quantity{}
		}
		mc.Status.Capacity[v1.ResourceCPU] = node.Status.Capacity[v1.ResourceCPU]
		if mc.Status.Allocatable == nil {
			mc.Status.Allocatable = map[v1.ResourceName]resource.Quantity{}
		}
		mc.Status.Allocatable[v1.ResourceMemory] = node.Status.Allocatable[v1.ResourceMemory]
		mc.Status.Version = mct.ManagedClusterVersion{Kubernetes: node.Status.NodeInfo.KubeletVersion}
	}
	if err := r.Client.Update(ctx, &mc); err != nil {
		//todo log error
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.clusterMap = make(map[string]mct.ManagedCluster)
	r.fleetId = string(uuid.NewUUID())

	return ctrl.NewControllerManagedBy(mgr).
		For(&mct.ManagedCluster{}).
		Complete(r)
}

// InFleet yields a bool indicating if the ManagedCluster is a part of a fleet by checking the details of the Lease.
func (r *ManagedClusterReconciler) InFleet(memberCluster mct.ManagedCluster, nameSpaceName string) (bool, error) {
	// todo
	return false, nil
}

// ReconcileManagedClusterFleetStatus reconciles the managed cluster's state with respect to its membership to the fleet.
func (r *ManagedClusterReconciler) ReconcileManagedClusterFleetStatus(kubeClient *kubernetes.Clientset) error {
	//Fleet Logic
	// ToDo : No, this would prevent any reconciliation of a fleet after it was joined.
	//exists, err := r.InFleet(mc, req.NamespacedName.Name)
	//if err != nil || exists {
	//	return ctrl.Result{}, err
	//}

	// TODO: After creating more fields for lease status, add here.
	//newLease := v1alpha1.MemberClusterLease{
	//	TypeMeta:   metav1.TypeMeta{Kind: "MemberClusterLease", APIVersion: "v1alpha1"},
	//	ObjectMeta: metav1.ObjectMeta{},
	//	Spec:       v1alpha1.MemberClusterLeaseSpec{FleetID: r.fleetId},
	//}
	//mc.Lease = newLease
	//r.clusterMap[req.NamespacedName.Name] = mc
	return nil
}

// ConstructManagedClusterKubeClientFromClientConfig constructs a kubernetes client with provided client configuration credentials
func (r *ManagedClusterReconciler) ConstructManagedClusterKubeClientFromClientConfig(clientConfigs []mct.ClientConfig) (*kubernetes.Clientset, error) {
	var clientConfig mct.ClientConfig
	var encodedKubeConfig string

	if validConfig, err := r.GetFirstValidClientConfig(clientConfigs); err != nil {
		return nil, err
	} else {
		clientConfig = validConfig
	}

	if data, err := r.GetMemberClusterKubeConfig(clientConfig.SecretRef, managedClusterKubeConfigSecretNamespace); err != nil {
		return nil, err
	} else {
		encodedKubeConfig = data
	}

	if restConfig, err := r.ConstructRestConfigFromKubeConfigSecret(encodedKubeConfig); err != nil {
		return nil, err
	} else {
		if kubeClient, err := r.ConstructKubeClientFromRestConfig(*restConfig); err != nil {
			return nil, err
		} else {
			return kubeClient, nil
		}
	}
}

// GetFirstValidClientConfig selects the appropriate client configuration to be used in kubernetes client construction.
func (r *ManagedClusterReconciler) GetFirstValidClientConfig(clientConfigs []mct.ClientConfig) (mct.ClientConfig, error) {
	if len(clientConfigs) == 0 {
		return mct.ClientConfig{}, errorMissingManagedClusterClientConfig
	}
	// Find and return first value ClientConfig.
	// Todo - What logic should be used here to be selective?
	for i, value := range clientConfigs {
		if (len(value.URL) > 0) && (len(value.SecretRef) > 0) {
			return clientConfigs[i], nil
		}
	}

	return mct.ClientConfig{}, errorMissingManagedClusterClientConfig
}

// GetMemberClusterKubeConfig retrieves the encoded KubeConfig string that is stored within a secret.
func (r *ManagedClusterReconciler) GetMemberClusterKubeConfig(secretName string, secretNamespace string) (string, error) {
	var secret v1.Secret
	namespacedName := types.NamespacedName{Namespace: secretNamespace, Name: secretName}
	if err := r.Client.Get(context.Background(), namespacedName, &secret); err != nil {
		return "", err
	}

	kubeconfig, ok := secret.Data[kubeConfigDataKey]
	if !ok || len(kubeconfig) == 0 {
		return "", errorMissingManagedClusterKubeConfigSecret
	}

	return string(kubeconfig), nil
}

// ConstructRestConfigFromKubeConfigSecret constructs a configuration structure used by kubernetes client construction.
func (r *ManagedClusterReconciler) ConstructRestConfigFromKubeConfigSecret(encodedKubeConfig string) (*rest.Config, error) {
	restConfig, err := clientcmd.BuildConfigFromKubeconfigGetter(
		"",
		func() (*clientcmdapi.Config, error) {
			return clientcmd.Load([]byte(encodedKubeConfig))
		})

	if err != nil {
		return nil, err
	} else {
		return restConfig, nil
	}
}

// ConstructKubeClientFromRestConfig constructs a kubernetes client for a given configuration.
func (r *ManagedClusterReconciler) ConstructKubeClientFromRestConfig(config rest.Config) (*kubernetes.Clientset, error) {
	if kubeClient, err := kubernetes.NewForConfig(&config); err != nil {
		return kubeClient, errorInvalidKubeConfig
	} else {
		return kubeClient, nil
	}
}
