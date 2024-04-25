package controller

import (
	"context"

	"github.com/go-logr/logr"
	topolvm "github.com/syself/csi-topolvm"
	topolvmv1 "github.com/syself/csi-topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client           client.Client
	skipNodeFinalize bool
}

// NewNodeReconciler returns NodeReconciler.
func NewNodeReconciler(client client.Client, skipNodeFinalize bool) *NodeReconciler {
	return &NodeReconciler{
		client:           client,
		skipNodeFinalize: skipNodeFinalize,
	}
}

//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile finalize Node
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)

	// your logic here
	node := &corev1.Node{}
	err := r.client.Get(ctx, req.NamespacedName, node)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	if node.DeletionTimestamp == nil {
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(node, topolvm.GetNodeFinalizer()) {
		return ctrl.Result{}, nil
	}

	if result, err := r.doFinalize(ctx, log, node); result.Requeue || err != nil {
		return result, err
	}

	node2 := node.DeepCopy()
	controllerutil.RemoveFinalizer(node2, topolvm.GetNodeFinalizer())
	if err := r.client.Patch(ctx, node2, client.MergeFrom(node)); err != nil {
		log.Error(err, "failed to remove finalizer", "name", node.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *NodeReconciler) targetStorageClasses(ctx context.Context) (map[string]bool, error) {
	var scl storagev1.StorageClassList
	if err := r.client.List(ctx, &scl); err != nil {
		return nil, err
	}

	targets := make(map[string]bool)
	for _, sc := range scl.Items {
		if sc.Provisioner != topolvm.GetPluginName() {
			continue
		}
		targets[sc.Name] = true
	}
	return targets, nil
}

func (r *NodeReconciler) doFinalize(ctx context.Context, log logr.Logger, nodeObj *corev1.Node) (ctrl.Result, error) {
	if r.skipNodeFinalize {
		log.Info("skipping node finalize")
		return ctrl.Result{}, nil
	}

	scs, err := r.targetStorageClasses(ctx)
	if err != nil {
		log.Error(err, "unable to fetch StorageClass")
		return ctrl.Result{}, err
	}

	var pvcs corev1.PersistentVolumeClaimList
	err = r.client.List(ctx, &pvcs, client.MatchingFields{keySelectedNode: nodeObj.Name})
	if err != nil {
		log.Error(err, "unable to fetch PersistentVolumeClaimList",
			"keySelectedNode", keySelectedNode,
			"nodeObj.Name", nodeObj.Name) // ööö
		/// "Index with name field:metadata.annotations.selected-node does not exist
		return ctrl.Result{}, err
	}

	for _, pvc := range pvcs.Items {
		if pvc.Spec.StorageClassName == nil {
			continue
		}
		if !scs[*pvc.Spec.StorageClassName] {
			continue
		}

		err = r.client.Delete(ctx, &pvc)
		if err != nil {
			log.Error(err, "unable to delete PVC", "name", pvc.Name, "namespace", pvc.Namespace)
			return ctrl.Result{}, err
		}
		log.Info("deleted PVC", "name", pvc.Name, "namespace", pvc.Namespace)
	}

	lvList := &topolvmv1.LogicalVolumeList{}
	err = r.client.List(ctx, lvList, client.MatchingFields{keyLogicalVolumeProviderID: nodeObj.Spec.ProviderID})
	if err != nil {
		log.Error(err, "failed to get LogicalVolumes")
		return ctrl.Result{}, err
	}
	for _, lv := range lvList.Items {
		err = r.cleanupLogicalVolume(ctx, log, &lv)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *NodeReconciler) cleanupLogicalVolume(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	if controllerutil.ContainsFinalizer(lv, topolvm.GetLogicalVolumeFinalizer()) {
		lv2 := lv.DeepCopy()
		if lv2.Annotations == nil {
			lv2.Annotations = make(map[string]string)
		}
		// Flag the LV as pending deletion, so the LogicalVolumeReconciler doesn't re-add the finalizer before it sees the deletion
		lv2.Annotations[topolvm.GetLVPendingDeletionKey()] = "true"
		controllerutil.RemoveFinalizer(lv2, topolvm.GetLogicalVolumeFinalizer())
		if err := r.client.Patch(ctx, lv2, client.MergeFrom(lv)); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("LogicalVolume is already deleted", "name", lv.Name)
				return nil
			}
			log.Error(err, "failed to patch LogicalVolume", "name", lv.Name)
			return err
		}
	}

	if err := r.client.Delete(ctx, lv); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("LogicalVolume is already deleted", "name", lv.Name)
			return nil
		}
		log.Error(err, "failed to delete LogicalVolume", "name", lv.Name)
		return err
	}
	log.Info("deleted LogicalVolume", "name", lv.Name)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.PersistentVolumeClaim{}, keySelectedNode, func(o client.Object) []string {
		return []string{o.(*corev1.PersistentVolumeClaim).Annotations[AnnSelectedNode]}
	})
	if err != nil {
		return err
	}
	err = mgr.GetFieldIndexer().IndexField(ctx, &topolvmv1.LogicalVolume{}, keyLogicalVolumeProviderID, func(o client.Object) []string {
		return []string{o.(*topolvmv1.LogicalVolume).Spec.ProviderID}
	})
	if err != nil {
		return err
	}

	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		Named("node-controller").
		WatchesMetadata(&corev1.Node{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
