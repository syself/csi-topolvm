package controller

import (
	"context"
	"fmt"

	internalController "github.com/syself/csi-topolvm/internal/controller"
	"github.com/syself/csi-topolvm/pkg/lvmd/proto"
	"github.com/syself/csi-topolvm/pkg/node"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupLogicalVolumeReconcilerWithServices creates LogicalVolumeReconciler and sets up with manager.
func SetupLogicalVolumeReconcilerWithServices(mgr ctrl.Manager, client client.Client, nodeName string, vgService proto.VGServiceClient, lvService proto.LVServiceClient) error {
	providerID, err := node.GetProviderIDByNodeName(context.Background(), client, nodeName)
	if err != nil {
		return fmt.Errorf("GetProviderID failed for node %q: %w", nodeName, err)
	}
	reconciler, err := internalController.NewLogicalVolumeReconcilerWithServices(client, nodeName, providerID, vgService, lvService)
	if err != nil {
		return err
	}
	return reconciler.SetupWithManager(mgr)
}
