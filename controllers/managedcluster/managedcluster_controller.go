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
	"github.com/Ealianis/caravel-mcm/api/cluster/v1alpha1"
	mclr "github.com/Ealianis/caravel-mcm/api/cluster/v1alpha1/memberclusterlease"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logger "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ManagedClusterReconciler reconciles a ManagedCluster object
type ManagedClusterReconciler struct {
	Client       client.Client
	CoreV1Client corev1.CoreV1Interface
	Scheme       *runtime.Scheme
	fleetId      string
}

const (
	HubFleetId                             = "HubFleetIdToDo"
	KubeConfigDataKey                      = "kubeconfig"
	MemberClusterKubeConfigSecretNamespace = "member-cluster-kubeconfigs"
	MemberClusterLeaseNamespace            = "memberships"
	MemberClusterLeaseName                 = "fleet-lease"
)

var (
	errorFailedKubeConfig                      = errors.New("the ManagedCluster's KubeConfig was unable to create a client")
	errorUnableToFindManagedClusterResource    = errors.New("the ManagedCluster resource could not be retrieved")
	errorManagedClusterDataPurgeFailure        = errors.New("the ManagedCluster's data could not be purged")
	errorManagedClusterJoinedToDifferentFleet  = errors.New("the ManagedCluster belongs to another fleet")
	errorManagedClusterReconcileFailure        = errors.New("the ManagedCluster could not be reconciled")
	errorMissingManagedClusterClientConfig     = errors.New("the ManagedCluster does not have a valid client configuration")
	errorMissingManagedClusterKubeConfigSecret = errors.New("the ManagedCluster is missing a KubeConfig secret")

	memberClusterLeaseObjectKey = client.ObjectKey{Namespace: MemberClusterLeaseNamespace, Name: MemberClusterLeaseName}
	memberClusterLeaseGVK       = schema.GroupVersionKind{Group: "cluster.aks-caravel.mcm", Version: "v1alpha1", Kind: "MemberClusterLease"}

	availableCondition = metav1.Condition{
		Type: v1alpha1.ManagedClusterConditionAvailable,
	}

	joinedCondition = metav1.Condition{
		Type: v1alpha1.ManagedClusterConditionJoined,
	}
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
	log := logger.FromContext(ctx)

	var mc v1alpha1.ManagedCluster
	err := r.Client.Get(ctx, req.NamespacedName, &mc)

	if err != nil {
		log.Error(errorUnableToFindManagedClusterResource, err.Error())
		err := r.PurgeMemberClusterData(ctx, req)
		if err != nil {
			log.Error(err, err.Error())
		}
		return ctrl.Result{}, nil
	}

	cs, err := r.ConstructClientSetFromClientConfig(mc.Spec.ManagedClusterClientConfigs)
	if err != nil {
		log.Error(err, "")
		return ctrl.Result{}, err
	}

	err = r.ReconcileManagedClusterFleetStatus(ctx, req, mc)
	if err != nil {
		log.Error(err, err.Error())
	}

	mc.Status.Capacity = map[v1.ResourceName]resource.Quantity{}
	mc.Status.Allocatable = map[v1.ResourceName]resource.Quantity{}

	nodeList, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		availableCondition.Status = metav1.ConditionFalse
		availableCondition.Reason = err.Error()
		availableCondition.Message = "Cannot retrieve Resource information from the managed cluster."
		availableCondition.LastTransitionTime = metav1.Now()
		meta.SetStatusCondition(&mc.Status.Conditions, availableCondition)
	} else {
		for _, node := range nodeList.Items {
			mc.Status.Capacity[v1.ResourceCPU] = node.Status.Capacity[v1.ResourceCPU]
			mc.Status.Capacity[v1.ResourceMemory] = node.Status.Capacity[v1.ResourceMemory]
			mc.Status.Allocatable[v1.ResourceCPU] = node.Status.Allocatable[v1.ResourceCPU]
			mc.Status.Allocatable[v1.ResourceMemory] = node.Status.Allocatable[v1.ResourceMemory]
			mc.Status.Version = v1alpha1.ManagedClusterVersion{Kubernetes: node.Status.NodeInfo.KubeletVersion}
		}

		availableCondition.Status = metav1.ConditionTrue
		availableCondition.LastTransitionTime = metav1.Now()
		meta.SetStatusCondition(&mc.Status.Conditions, availableCondition)
	}

	if err := r.Client.Status().Update(ctx, &mc); err != nil {
		//todo log error
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ManagedClusterReconciler) UpdateManagedCluster(ctx context.Context, managedCluster v1alpha1.ManagedCluster) error {
	if err := r.Client.Status().Update(ctx, &managedCluster); err != nil {
		return err
	} else {
		return nil
	}
}

// ReconcileManagedClusterFleetStatus reconciles the target cluster's state with respect to its membership to the fleet.
func (r *ManagedClusterReconciler) ReconcileManagedClusterFleetStatus(ctx context.Context, req reconcile.Request, mc v1alpha1.ManagedCluster) error {
	// Construct a client to communicate with the target cluster.
	c, err := r.ConstructCRClientFromClientConfigs(mc.Spec.ManagedClusterClientConfigs)
	if err != nil {
		availableCondition.Status = metav1.ConditionFalse
		availableCondition.Reason = err.Error()
		availableCondition.Message = "A client could not be established for the target cluster."
		availableCondition.LastTransitionTime = metav1.Now()
		meta.SetStatusCondition(&mc.Status.Conditions, availableCondition)

		return err
	}

	c.Scheme().AddKnownTypeWithName(memberClusterLeaseGVK, &v1alpha1.MemberClusterLease{})

	// Reconcile member cluster lease
	var lease v1alpha1.MemberClusterLease
	if err := c.Get(ctx, memberClusterLeaseObjectKey, &lease); err != nil {
		if k8sErrors.IsNotFound(err) {
			newLease := GenerateNewLease()

			if cerr := c.Create(ctx, newLease); cerr == nil {
				joinedCondition.Status = metav1.ConditionTrue
				joinedCondition.Message = "Cluster is joined to the fleet."
				joinedCondition.LastTransitionTime = metav1.Now()
				meta.SetStatusCondition(&mc.Status.Conditions, joinedCondition)

				return nil
			}
		}

		joinedCondition.Status = metav1.ConditionUnknown
		joinedCondition.Reason = err.Error()
		joinedCondition.Message = "An error occurred when attempting to retrieve the MemberClusterLease from the target cluster."
		joinedCondition.LastTransitionTime = metav1.Now()
		meta.SetStatusCondition(&mc.Status.Conditions, joinedCondition)

		return err
	} else {
		availableCondition.Status = metav1.ConditionTrue
		availableCondition.LastTransitionTime = metav1.Now()
		meta.SetStatusCondition(&mc.Status.Conditions, availableCondition)

		// A lease was found. Is it ours?
		if lease.Spec.FleetID == HubFleetId {
			lease.Spec.LastLeaseRenewTime = metav1.Now()
			if err = c.Update(ctx, &lease); err == nil {
				joinedCondition.Status = metav1.ConditionTrue
				joinedCondition.LastTransitionTime = metav1.Now()
				meta.SetStatusCondition(&mc.Status.Conditions, joinedCondition)

				return nil
			}

			return err
		} else {
			// Cluster belongs to a different fleet. We should remove the data / secret from the hub cluster.
			joinedCondition.Status = metav1.ConditionFalse
			joinedCondition.Message = "Member cluster belongs to a different fleet."
			joinedCondition.LastTransitionTime = metav1.Now()
			meta.SetStatusCondition(&mc.Status.Conditions, joinedCondition)
			if err := r.PurgeMemberClusterData(ctx, req); err != nil {

				return err
			}

		}

		return nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.fleetId = string(uuid.NewUUID())

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ManagedCluster{}).
		Complete(r)
}

// ConstructCRClientFromClientConfigs constructs a controller-runtime/client using the ManagedCluster's ClientConfigs
func (r *ManagedClusterReconciler) ConstructCRClientFromClientConfigs(clientConfigs []v1alpha1.ClientConfig) (client.Client, error) {
	if rc, err := r.ConstructRestConfigFromManagedClusterClientConfigs(clientConfigs); err != nil {
		return nil, errorMissingManagedClusterClientConfig

	} else {
		if c, err := client.New(rc, client.Options{Scheme: r.Scheme}); err != nil {
			return nil, err

		} else {
			return c, nil
		}
	}
}

// ConstructClientSetFromClientConfig constructs a kubernetes client with provided client configuration credentials
func (r *ManagedClusterReconciler) ConstructClientSetFromClientConfig(clientConfigs []v1alpha1.ClientConfig) (*kubernetes.Clientset, error) {
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

// ConstructRestConfigFromManagedClusterClientConfigs constructs a rest.Config structure with the provided v1alpha1.ClientConfig.
func (r *ManagedClusterReconciler) ConstructRestConfigFromManagedClusterClientConfigs(clientConfigs []v1alpha1.ClientConfig) (*rest.Config, error) {
	var clientConfig v1alpha1.ClientConfig
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
func (r *ManagedClusterReconciler) GetFirstValidClientConfig(clientConfigs []v1alpha1.ClientConfig) (v1alpha1.ClientConfig, error) {
	if len(clientConfigs) == 0 {
		return v1alpha1.ClientConfig{}, errorMissingManagedClusterClientConfig
	}
	// Find and return first value ClientConfig.
	// Todo - What logic should be used here to be selective?
	for i, value := range clientConfigs {
		if (len(value.URL) > 0) && (len(value.SecretRef) > 0) {
			return clientConfigs[i], nil
		}
	}

	return v1alpha1.ClientConfig{}, errorMissingManagedClusterClientConfig
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

func GenerateNewLease() *v1alpha1.MemberClusterLease {
	return &v1alpha1.MemberClusterLease{
		TypeMeta: metav1.TypeMeta{
			Kind:       mclr.Kind,
			APIVersion: mclr.GroupVersion,
		},

		ObjectMeta: metav1.ObjectMeta{
			Name:      MemberClusterLeaseName,
			Namespace: MemberClusterLeaseNamespace,
		},

		Spec: v1alpha1.MemberClusterLeaseSpec{
			FleetID:            HubFleetId,
			LastLeaseRenewTime: metav1.Now(),
			LastJoinTime:       metav1.Now(),
		},
		Status: v1alpha1.MemberClusterLeaseStatus{},
	}
}

func (r *ManagedClusterReconciler) PurgeMemberClusterData(ctx context.Context, req ctrl.Request) error {
	var secret v1.Secret
	namespacedName := types.NamespacedName{Namespace: MemberClusterKubeConfigSecretNamespace, Name: "member-cluster-" + req.Name + "-kubeconfig"}

	if err := r.Client.Get(context.Background(), namespacedName, &secret); err == nil {
		deleteErr := r.Client.Delete(ctx, &secret)

		if deleteErr != nil {
			logger.Log.Error(deleteErr, errorManagedClusterDataPurgeFailure.Error())

			return nil
		}
	}

	logger.Log.Info("Data for member cluster" + req.Name + "has been purged.")

	return nil
}
