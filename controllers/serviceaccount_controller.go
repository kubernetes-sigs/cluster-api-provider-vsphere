/*
Copyright 2021 The Kubernetes Authors.

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

package controllers

import (
	_context "context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/builder"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	vmwarecontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	controllerName             = "provider-serviceaccount-controller"
	kindProviderServiceAccount = "ProviderServiceAccount"
	systemServiceAccountPrefix = "system.serviceaccount"
)

// AddToManager adds this package's controller to the provided manager.
func AddServiceAccountProviderControllerToManager(ctx *context.ControllerManagerContext, mgr manager.Manager) error {
	var (
		controlledType     = &vmwarev1.ProviderServiceAccount{}
		controlledTypeName = reflect.TypeOf(controlledType).Elem().Name()
		//controlledTypeGVK   = infrav1.GroupVersion.WithKind(controlledTypeName)
		//controlledTypeGVK = vmwarev1.GroupVersion.WithKind(controlledTypeName)
		controllerNameShort = fmt.Sprintf("%s-supervisor-controller", strings.ToLower(controlledTypeName))
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", ctx.Namespace, ctx.Name, controllerNameShort)
	)

	controllerContext := &context.ControllerContext{
		ControllerManagerContext: ctx,
		Name:                     controllerNameShort,
		Recorder:                 record.New(mgr.GetEventRecorderFor(controllerNameLong)),
		Logger:                   ctx.Logger.WithName(controllerNameShort),
	}
	// When watching a VSphereCluster, we only care about Create and Update events.
	clusterPredicates := predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return true },
		// Reconcile on update. Don't test for equality as the DefaultSyncTime reconciles happen through this path
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	// When watching a Cluster, we only care about Update events.
	// Should not be necessary to watch Create events since we watch VSphereCluster Create and
	//  there are no circumstances in which a VSphereCluster and Cluster can be created separately
	capiClusterPredicates := predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			for _, ownerRef := range e.ObjectNew.GetOwnerReferences() {
				if ownerRef.Name == e.ObjectNew.GetName() {
					// We don't have this check in VSphereCluster Update ensuring that every DefaultTimeSync
					// we're guaranteed to have at least one Reconcile.
					// If we didn't have this check here, we'd get at least two Reconciles every DefaultTimeSync
					return e.ObjectOld.GetResourceVersion() != e.ObjectNew.GetResourceVersion()
				}
			}
			return false
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	vsphereCluster := &vmwarev1.VSphereCluster{}
	r := ServiceAccountReconciler{
		ControllerContext: controllerContext,
	}

	return ctrl.NewControllerManagedBy(mgr).For(controlledType).
		Watches(
			&source.Kind{Type: &vmwarev1.VSphereCluster{}}, &handler.EnqueueRequestForObject{},
		).
		// Watch a ProviderServiceAccount
		Watches(
			&source.Kind{Type: &vmwarev1.ProviderServiceAccount{}}, &handler.EnqueueRequestForOwner{
				OwnerType:    &vmwarev1.VSphereCluster{},
				IsController: true,
			}).
		Watches(
			&source.Kind{Type: &corev1.ServiceAccount{}},
			handler.EnqueueRequestsFromMapFunc(requestMapper{ctx: controllerContext.ControllerManagerContext}.Map),
		).
		Watches(
			&source.Kind{Type: &rbacv1.Role{}},
			handler.EnqueueRequestsFromMapFunc(requestMapper{ctx: controllerContext.ControllerManagerContext}.Map),
		).
		Watches(
			&source.Kind{Type: &rbacv1.RoleBinding{}},
			handler.EnqueueRequestsFromMapFunc(requestMapper{ctx: controllerContext.ControllerManagerContext}.Map),
		).
		WithEventFilter(clusterPredicates).
		// watch the CAPI cluster
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}}, &handler.EnqueueRequestForOwner{
				OwnerType:    vsphereCluster,
				IsController: true,
			}).
		WithEventFilter(capiClusterPredicates).
		Complete(r)
}

type requestMapper struct {
	ctx *context.ControllerManagerContext
}

// TODO: [Aarti] - maybe remove the requestMapper, and directly call a getVsphereCluster in the Watch.
func (d requestMapper) Map(o client.Object) []reconcile.Request {
	// If the watched object [role|rolebinding|serviceaccount] is owned by this providerserviceaccount controller, then
	// lookup the vsphere cluster that owns the providerserviceaccount object that needs to be queued. We do this because
	// this controller is effectively a vsphere controller which reconciles it's dependent providerserviceaccounts.
	ownerRef := metav1.GetControllerOf(o)
	if ownerRef != nil && ownerRef.Kind == kindProviderServiceAccount {
		key := types.NamespacedName{Namespace: o.GetNamespace(), Name: ownerRef.Name}
		return getVSphereCluster(d.ctx, key)
	}
	return nil
}

func getVSphereCluster(ctx *context.ControllerManagerContext, pSvcAccountKey types.NamespacedName) []reconcile.Request {
	pSvcAccount := &vmwarev1.ProviderServiceAccount{}
	if err := ctx.Client.Get(ctx, pSvcAccountKey, pSvcAccount); err != nil {
		return nil
	}

	vsphereClusterRef := pSvcAccount.Spec.Ref
	if vsphereClusterRef == nil || vsphereClusterRef.Name == "" {
		return nil
	}
	key := client.ObjectKey{Namespace: pSvcAccount.Namespace, Name: vsphereClusterRef.Name}
	return []reconcile.Request{{NamespacedName: key}}
}

func NewServiceAccountReconciler() builder.Reconciler {
	return ServiceAccountReconciler{}
}

type ServiceAccountReconciler struct {
	*context.ControllerContext
}

func (r ServiceAccountReconciler) Reconcile(ctx _context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	r.ControllerContext.Logger.V(4).Info("Starting Reconcile")

	// Get the vspherecluster for this request.
	vsphereCluster := &vmwarev1.VSphereCluster{}
	clusterKey := client.ObjectKey{Namespace: req.Namespace, Name: req.Name}
	if err := r.Client.Get(r, clusterKey, vsphereCluster); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.Info("Cluster not found, won't reconcile", "cluster", clusterKey)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(vsphereCluster, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			vsphereCluster.GroupVersionKind(),
			vsphereCluster.Namespace,
			vsphereCluster.Name)
	}

	// Create the cluster context for this request.
	clusterContext := &vmwarecontext.ClusterContext{
		ControllerContext: r.ControllerContext,
		VSphereCluster:    vsphereCluster,
		Logger:            r.Logger.WithName(req.Namespace).WithName(req.Name),
		PatchHelper:       patchHelper,
	}

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		if err := clusterContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			} else {
				clusterContext.Logger.Error(err, "patch failed", "cluster", clusterContext.String())
			}
		}
	}()

	// This type of controller doesn't care about delete events.
	if !vsphereCluster.DeletionTimestamp.IsZero() {
		return r.ReconcileDelete(clusterContext)
	}

	// We cannot proceed until we are able to access the target cluster. Until
	// then just return a no-op and wait for the next sync. This will occur when
	// the Cluster's status is updated with a reference to the secret that has
	// the Kubeconfig data used to access the target cluster.
	_, err = remote.NewClusterClient(clusterContext, clusterContext.Cluster.Name, clusterContext.Client, clusterKey)
	if err != nil {
		clusterContext.Logger.Info("The control plane is not ready yet", "err", err)
		return reconcile.Result{RequeueAfter: clusterNotReadyRequeueTime}, nil
	}

	//// All controllers should wait until the PSP are created and bind successfully by DefaultPSP controller.
	//for _, requiredComponent := range r.RequiredComponents() {
	//	if requiredComponent == DefaultPSP {
	//		// Do not reconcile until the default PSP are created.
	//		pspStatus := status.FindAddonStatusByType(clusterContext.Cluster, vmwarev1.PSP)
	//		if pspStatus == nil || status.IsFalseCondition(pspStatus, tkgv1.ProvisionedCondition) {
	//			clusterContext.Logger.Info("Skipping reconcile until PSP Addon ProvisionedCondition is true",
	//				"cluster", clusterContext.String())
	//			// No need to requeue because the change of Cluster.Status.Addons.PSP.Applied will trigger reconcile.
	//			return reconcile.Result{}, nil
	//		}
	//	}
	//}

	// Defer to the Reconciler for reconciling a non-delete event.
	return r.ReconcileNormal(clusterContext)
}

func (r ServiceAccountReconciler) ReconcileDelete(ctx *vmwarecontext.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.V(4).Info("Reconciling deleting Provider ServiceAccounts", "cluster", ctx.Cluster.Name)

	pSvcAccounts, err := getProviderServiceAccounts(ctx)
	if err != nil {
		ctx.Logger.Error(err, "Error fetching provider serviceaccounts")
		return reconcile.Result{}, err
	}

	for _, pSvcAccount := range pSvcAccounts {
		// Delete entries for configmap with serviceaccount
		if err := r.deleteServiceAccountConfigMap(ctx, pSvcAccount); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "unable to delete configmap entry for provider serviceaccount %s", pSvcAccount.Name)
		}
	}

	return reconcile.Result{}, nil
}

// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=providerserviceaccounts,verbs=get;list;watch;
// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=providerserviceaccounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete

func (r ServiceAccountReconciler) ReconcileNormal(ctx *vmwarecontext.ClusterContext) (_ reconcile.Result, reterr error) {
	ctx.Logger.V(4).Info("Reconciling Provider ServiceAccount", "cluster", ctx.Cluster.Name)

	defer func() {
		if reterr != nil {
			conditions.MarkFalse(ctx.Cluster, vmwarev1.ProviderServiceAccountsReadyCondition, vmwarev1.ProviderServiceAccountsReconciliationFailedReason,
				clusterv1.ConditionSeverityWarning, reterr.Error())
		} else {
			conditions.MarkTrue(ctx.Cluster, vmwarev1.ProviderServiceAccountsReadyCondition)
		}
	}()

	pSvcAccounts, err := getProviderServiceAccounts(ctx)
	if err != nil {
		ctx.Logger.Error(err, "Error fetching provider serviceaccounts")
		return reconcile.Result{}, err
	}
	err = r.ensureProviderServiceAccounts(ctx, pSvcAccounts)
	if err != nil {
		ctx.Logger.Error(err, "Error ensuring provider serviceaccounts")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// Ensure service accounts from provider spec is created
func (r ServiceAccountReconciler) ensureProviderServiceAccounts(ctx *vmwarecontext.ClusterContext, pSvcAccounts []vmwarev1.ProviderServiceAccount) error {

	for _, pSvcAccount := range pSvcAccounts {
		// 1. Create service accounts by the name specified in Provider Spec
		if err := r.ensureServiceAccount(ctx, pSvcAccount); err != nil {
			return errors.Wrapf(err, "unable to create provider serviceaccount %s", pSvcAccount.Name)
		}

		// 2. Update configmap with serviceaccount
		if err := r.ensureServiceAccountConfigMap(ctx, pSvcAccount); err != nil {
			return errors.Wrapf(err, "unable to sync configmap for provider serviceaccount %s", pSvcAccount.Name)
		}

		// 3. Create the associated role for the service account
		if err := r.ensureRole(ctx, pSvcAccount); err != nil {
			return errors.Wrapf(err, "unable to create role for provider serviceaccount %s", pSvcAccount.Name)
		}

		// 4. Create the associated roleBinding for the service account
		if err := r.ensureRoleBinding(ctx, pSvcAccount); err != nil {
			return errors.Wrapf(err, "unable to create rolebinding for provider serviceaccount %s", pSvcAccount.Name)
		}

		// 5. Sync the service account with the target
		if err := r.syncServiceAccountSecret(ctx, pSvcAccount); err != nil {
			return errors.Wrapf(err, "unable to sync secret for provider serviceaccount %s", pSvcAccount.Name)
		}
	}
	return nil
}

func (r ServiceAccountReconciler) ensureServiceAccount(ctx *vmwarecontext.ClusterContext, pSvcAccount vmwarev1.ProviderServiceAccount) error {
	svcAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServiceAccountName(pSvcAccount),
			Namespace: pSvcAccount.Namespace,
		},
	}
	logger := ctx.Logger.WithValues("providerserviceaccount", pSvcAccount.Name, "serviceaccount", svcAccount.Name)
	err := controllerutil.SetControllerReference(&pSvcAccount, &svcAccount, ctx.Scheme)
	if err != nil {
		return err
	}
	logger.V(4).Info("Creating service account")
	err = ctx.Client.Create(ctx, &svcAccount)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		// Note: We skip updating the service account because the token controller updates the service account with a
		// secret and we don't want to overwrite it with an empty secret.
		return err
	}
	return nil
}

func (r ServiceAccountReconciler) ensureRole(ctx *vmwarecontext.ClusterContext, pSvcAccount vmwarev1.ProviderServiceAccount) error {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getRoleName(pSvcAccount),
			Namespace: pSvcAccount.Namespace,
		},
	}
	logger := ctx.Logger.WithValues("providerserviceaccount", pSvcAccount.Name, "role", role.Name)
	logger.V(4).Info("Creating or updating role")
	_, err := controllerutil.CreateOrUpdate(ctx, ctx.Client, &role, func() error {
		if err := controllerutil.SetControllerReference(&pSvcAccount, &role, ctx.Scheme); err != nil {
			return err
		}
		role.Rules = pSvcAccount.Spec.Rules
		return nil
	})
	return err
}

func (r ServiceAccountReconciler) ensureRoleBinding(ctx *vmwarecontext.ClusterContext, pSvcAccount vmwarev1.ProviderServiceAccount) error {
	roleName := getRoleName(pSvcAccount)
	svcAccountName := getServiceAccountName(pSvcAccount)
	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getRoleBindingName(pSvcAccount),
			Namespace: pSvcAccount.Namespace,
		},
	}
	logger := ctx.Logger.WithValues("providerserviceaccount", pSvcAccount.Name, "rolebinding", roleBinding.Name)
	logger.V(4).Info("Creating or updating rolebinding")
	_, err := controllerutil.CreateOrUpdate(ctx, ctx.Client, &roleBinding, func() error {
		if err := controllerutil.SetControllerReference(&pSvcAccount, &roleBinding, ctx.Scheme); err != nil {
			return err
		}
		roleBinding.RoleRef = rbacv1.RoleRef{
			Name:     roleName,
			Kind:     "Role",
			APIGroup: rbacv1.GroupName,
		}
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				APIGroup:  "",
				Name:      svcAccountName,
				Namespace: pSvcAccount.Namespace},
		}
		return nil
	})
	return err
}

func (r ServiceAccountReconciler) syncServiceAccountSecret(ctx *vmwarecontext.ClusterContext, pSvcAccount vmwarev1.ProviderServiceAccount) error {
	logger := ctx.Logger.WithValues("providerserviceaccount", pSvcAccount.Name)
	logger.V(4).Info("Attempting to sync secret for provider service account")
	var svcAccount corev1.ServiceAccount
	err := ctx.Client.Get(ctx, types.NamespacedName{Name: getServiceAccountName(pSvcAccount), Namespace: pSvcAccount.Namespace}, &svcAccount)
	if err != nil {
		return err
	}
	// Check if token secret exists
	if len(svcAccount.Secrets) == 0 {
		// Note: We don't have to requeue here because we have a watch on the service account and the cluster should be reconciled
		// when a secret is added to the service account by the token controller.
		logger.Info("Skipping sync secret for provider service account: serviceaccount has no secrets", "serviceaccount", svcAccount.Name)
		return nil
	}

	// Choose the default secret
	secretRef := svcAccount.Secrets[0]
	logger.V(4).Info("Fetching secret for provider service account", "secret", secretRef.Name)
	var sourceSecret corev1.Secret
	err = ctx.Client.Get(ctx, types.NamespacedName{Name: secretRef.Name, Namespace: svcAccount.Namespace}, &sourceSecret)
	if err != nil {
		return err
	}

	// Create the target namespace if it is not existing
	targetNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: pSvcAccount.Spec.TargetNamespace,
		},
	}

	if err = ctx.Client.Get(ctx, client.ObjectKey{Name: pSvcAccount.Spec.TargetNamespace}, targetNamespace); err != nil {
		if apierrors.IsNotFound(err) {
			err = ctx.Client.Create(ctx, targetNamespace)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Note: We ignore the Secret type & annotations because they are created by the token controller and will not be valid in the target cluster.
	targetSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pSvcAccount.Spec.TargetSecretName,
			Namespace: pSvcAccount.Spec.TargetNamespace,
		},
	}
	logger.V(4).Info("Creating or updating secret in cluster", "namespace", targetSecret.Namespace, "name", targetSecret.Name)
	_, err = controllerutil.CreateOrUpdate(ctx, ctx.Client, targetSecret, func() error {
		targetSecret.Data = sourceSecret.Data
		return nil
	})
	return err
}

func (r ServiceAccountReconciler) getConfigMapAndBuffer(ctx *vmwarecontext.ClusterContext) (*corev1.ConfigMap, *corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}

	if err := ctx.Client.Get(ctx, GetCMNamespaceName(), configMap); err != nil {
		return nil, nil, err
	}

	configMapBuffer := &corev1.ConfigMap{}
	configMapBuffer.Name = configMap.Name
	configMapBuffer.Namespace = configMap.Namespace
	return configMapBuffer, configMap, nil
}

func (r ServiceAccountReconciler) deleteServiceAccountConfigMap(ctx *vmwarecontext.ClusterContext, svcAccount vmwarev1.ProviderServiceAccount) error {
	logger := ctx.Logger.WithValues("providerserviceaccount", svcAccount.Name)

	svcAccountName := getSystemServiceAccountFullName(svcAccount)
	configMapBuffer, configMap, err := r.getConfigMapAndBuffer(ctx)
	if err != nil {
		return err
	}
	if valid, exist := configMap.Data[svcAccountName]; !exist || valid != strconv.FormatBool(true) {
		// Service account name is not in the config map
		return nil
	}
	logger.Info("Deleting config map entry for provider service account")
	_, err = controllerutil.CreateOrUpdate(ctx, ctx.Client, configMapBuffer, func() error {
		configMapBuffer.Data = configMap.Data
		delete(configMapBuffer.Data, svcAccountName)
		return nil
	})
	return err
}

func (r ServiceAccountReconciler) ensureServiceAccountConfigMap(ctx *vmwarecontext.ClusterContext, svcAccount vmwarev1.ProviderServiceAccount) error {
	logger := ctx.Logger.WithValues("providerserviceaccount", svcAccount.Name)

	svcAccountName := getSystemServiceAccountFullName(svcAccount)
	configMapBuffer, configMap, err := r.getConfigMapAndBuffer(ctx)
	if err != nil {
		return err
	}
	if valid, exist := configMap.Data[svcAccountName]; exist && valid == strconv.FormatBool(true) {
		// Service account name is already in the config map
		return nil
	}
	logger.Info("Updating config map for provider service account")
	_, err = controllerutil.CreateOrUpdate(ctx, ctx.Client, configMapBuffer, func() error {
		configMapBuffer.Data = configMap.Data
		configMapBuffer.Data[svcAccountName] = "true"
		return nil
	})
	return err
}

func getProviderServiceAccounts(ctx *vmwarecontext.ClusterContext) ([]vmwarev1.ProviderServiceAccount, error) {
	var pSvcAccounts []vmwarev1.ProviderServiceAccount

	pSvcAccountList := vmwarev1.ProviderServiceAccountList{}
	if err := ctx.Client.List(ctx, &pSvcAccountList, client.InNamespace(ctx.Cluster.Namespace)); err != nil {
		return nil, err
	}

	for _, pSvcAccount := range pSvcAccountList.Items {
		// step to clean up the target secret in the guest cluster. Note: when the provider service account is deleted
		// all the associated [serviceaccount|role|rolebindings] are deleted. Hence, the bearer token in the target
		// secret will be rendered invalid. Still, it's a good practice to delete the secret that we created.
		if pSvcAccount.DeletionTimestamp != nil {
			continue
		}
		ref := pSvcAccount.Spec.Ref
		if ref != nil && ref.Name == ctx.Cluster.Name {
			pSvcAccounts = append(pSvcAccounts, pSvcAccount)
		}
	}
	return pSvcAccounts, nil
}

func getRoleName(pSvcAccount vmwarev1.ProviderServiceAccount) string {
	return pSvcAccount.Name
}

func getRoleBindingName(pSvcAccount vmwarev1.ProviderServiceAccount) string {
	return pSvcAccount.Name
}

func getServiceAccountName(pSvcAccount vmwarev1.ProviderServiceAccount) string {
	return pSvcAccount.Name
}

func getSystemServiceAccountFullName(pSvcAccount vmwarev1.ProviderServiceAccount) string {
	return fmt.Sprintf("%s.%s.%s", systemServiceAccountPrefix, getServiceAccountNamespace(pSvcAccount), getServiceAccountName(pSvcAccount))
}

func getServiceAccountNamespace(pSvcAccount vmwarev1.ProviderServiceAccount) string {
	return pSvcAccount.Namespace
}

// GetCMNamespaceName gets capi valid modifier configmap metadata
func GetCMNamespaceName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: os.Getenv("SERVICE_ACCOUNTS_CM_NAMESPACE"),
		Name:      os.Getenv("SERVICE_ACCOUNTS_CM_NAME"),
	}
}
