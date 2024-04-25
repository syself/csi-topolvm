package controller

import (
	internalController "github.com/syself/csi-topolvm/internal/controller"
	"github.com/syself/csi-topolvm/pkg/lvmd/proto"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupLogicalVolumeReconcilerWithServices creates LogicalVolumeReconciler and sets up with manager.
func SetupLogicalVolumeReconcilerWithServices(mgr ctrl.Manager, client ctrClient.Client, nodeName string, providerID string, vgService proto.VGServiceClient, lvService proto.LVServiceClient) error {
	reconciler, err := internalController.NewLogicalVolumeReconcilerWithServices(client, nodeName, providerID, vgService, lvService)
	if err != nil {
		return err
	}
	return reconciler.SetupWithManager(mgr)
}
