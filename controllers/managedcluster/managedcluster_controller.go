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
	mcl "github.com/Ealianis/caravel-mcm/api/cluster/v1alpha1/memberclusterlease"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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
	MemberClusterKubeConfigSecretNamespace = "member-cluster-kubeconfigs"
	KubeConfigDataKey                      = "kubeconfig"
)

var (
	errorFailedKubeConfig                      = errors.New("the ManagedCluster's KubeConfig was unable to create a client")
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

func (r *ManagedClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var mc mct.ManagedCluster
	err := r.Client.Get(ctx, req.NamespacedName, &mc)

	if err != nil {
		logger.Error(errorUnableToFindManagedClusterResource, "", nil)
		return ctrl.Result{}, errorUnableToFindManagedClusterResource
	}

	cs, err := r.ConstructClientSetFromClientConfig(mc.Spec.ManagedClusterClientConfigs)
	if err != nil {
		logger.Error(err, "", nil)
		return ctrl.Result{}, err
	}

	err = r.ReconcileManagedClusterFleetStatus(ctx, mc, logger)
	if err != nil {
		logger.Error(err, "", nil)
		return ctrl.Result{}, err
	}

	if mc.Status.Capacity == nil {
		mc.Status.Capacity = map[v1.ResourceName]resource.Quantity{}
	}
	if mc.Status.Allocatable == nil {
		mc.Status.Allocatable = map[v1.ResourceName]resource.Quantity{}
	}
	if mc.Status.Conditions == nil {
		mc.Status.Conditions = []v1.NodeCondition{}
	}
	nodeList, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	for _, node := range nodeList.Items {
		mc.Status.Conditions = append(mc.Status.Conditions, node.Status.Conditions...)
		mc.Status.Capacity[v1.ResourceCPU] = node.Status.Capacity[v1.ResourceCPU]
		mc.Status.Allocatable[v1.ResourceMemory] = node.Status.Allocatable[v1.ResourceMemory]
		mc.Status.Version = mct.ManagedClusterVersion{Kubernetes: node.Status.NodeInfo.KubeletVersion}
	}

	if err := r.Client.Status().Update(ctx, &mc); err != nil {
		//todo log error
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ManagedClusterReconciler) UpdateManagedCluster(ctx context.Context, managedCluster mct.ManagedCluster) error {
	if err := r.Client.Status().Update(ctx, &managedCluster); err != nil {
		return err
	} else {
		return nil
	}
}

// ReconcileManagedClusterFleetStatus reconciles the managed cluster's state with respect to its membership to the fleet.
func (r *ManagedClusterReconciler) ReconcileManagedClusterFleetStatus(ctx context.Context, mc mct.ManagedCluster, logger logr.Logger) error {
	c, err := r.ConstructCRClientFromClientConfigs(mc.Spec.ManagedClusterClientConfigs)
	if err != nil {
		_ = append(mc.Status.ClusterClaims, mct.ManagedClusterClaim{Name: "ConnectionState", Value: mct.ManagedClusterTaintUnreachable})
		if updateErr := r.UpdateManagedCluster(ctx, mc); updateErr != nil {
			err = updateErr
		}
		return err
	}

	c.Scheme().AddKnownTypes(mcl.SchemeGroupVersion, &mcl.MemberClusterLease{}, &mcl.MemberClusterLeaseList{})

	// Reconcile member cluster lease
	var lease mcl.MemberClusterLease
	err = c.Get(ctx, client.ObjectKey{Namespace: "default", Name: "clusterlease"}, &lease)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			newLease := GenerateNewLease()
			err = c.Create(ctx, &newLease)
			return err
		} else {
			return err
		}
	} else {
		// Lease found. This Fleet's lease?
		if lease.Spec.FleetID == "todo-fleetid" {
			lease.Spec.LastLeaseRenewTime = metav1.Now()
		} else {
			// Cluster belongs to a different fleet. We should remove the data / secret from the hub cluster.
			// Todo - Delete MC Secret using secret ref.
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.clusterMap = make(map[string]mct.ManagedCluster)
	r.fleetId = string(uuid.NewUUID())

	return ctrl.NewControllerManagedBy(mgr).
		For(&mct.ManagedCluster{}).
		Complete(r)
}

// ConstructCRClientFromClientConfigs constructs a controller-runtime/client using the ManagedCluster's ClientConfigs
func (r *ManagedClusterReconciler) ConstructCRClientFromClientConfigs(clientConfigs []mct.ClientConfig) (client.Client, error) {
	if rc, err := r.ConstructRestConfigFromManagedClusterClientConfigs(clientConfigs); err != nil {

		return nil, errors.New("TODO")
	} else {
		if c, err := client.New(rc, client.Options{}); err != nil {
			return nil, err
		} else {
			return c, nil
		}
	}
}

// ConstructClientSetFromClientConfig constructs a kubernetes client with provided client configuration credentials
func (r *ManagedClusterReconciler) ConstructClientSetFromClientConfig(clientConfigs []mct.ClientConfig) (*kubernetes.Clientset, error) {
	if rc, err := r.ConstructRestConfigFromManagedClusterClientConfigs(clientConfigs); err != nil {
		return nil, err
	} else {
		if kubeClient, err := kubernetes.NewForConfig(rc); err != nil {
			return kubeClient, errorFailedKubeConfig
		} else {
			return kubeClient, nil
		}
	}
}

//
func (r *ManagedClusterReconciler) ConstructRestConfigFromManagedClusterClientConfigs(clientConfigs []mct.ClientConfig) (*rest.Config, error) {
	var clientConfig mct.ClientConfig
	var encodedKubeConfig []byte

	if validConfig, err := r.GetFirstValidClientConfig(clientConfigs); err != nil {
		return nil, err
	} else {
		clientConfig = validConfig
	}

	if data, err := r.GetMemberClusterKubeConfig(clientConfig.SecretRef, MemberClusterKubeConfigSecretNamespace); err != nil {
		return nil, err
	} else {
		encodedKubeConfig = data
	}

	if restConfig, err := clientcmd.RESTConfigFromKubeConfig(encodedKubeConfig); err != nil {
		return nil, err
	} else {
		return restConfig, nil
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
func (r *ManagedClusterReconciler) GetMemberClusterKubeConfig(secretName string, secretNamespace string) ([]byte, error) {
	var secret v1.Secret
	namespacedName := types.NamespacedName{Namespace: secretNamespace, Name: secretName}
	if err := r.Client.Get(context.Background(), namespacedName, &secret); err != nil {
		return nil, err
	}

	kubeConfig, ok := secret.Data[KubeConfigDataKey]
	if !ok || len(kubeConfig) == 0 {
		return nil, errorMissingManagedClusterKubeConfigSecret
	}

	return kubeConfig, nil
}

func GenerateNewLease() mcl.MemberClusterLease {
	return mcl.MemberClusterLease{
		TypeMeta: metav1.TypeMeta{
			Kind:       mcl.Kind,
			APIVersion: mcl.GroupVersion,
		},

		ObjectMeta: metav1.ObjectMeta{
			Name:      "clusterlease",
			Namespace: "default",
		},

		Spec: mcl.MemberClusterLeaseSpec{
			FleetID:            "fleetidexample",
			LastLeaseRenewTime: metav1.Now(),
			LastJoinTime:       metav1.Now(),
		},
		Status: mcl.MemberClusterLeaseStatus{},
	}
}
