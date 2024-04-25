package lvmd

import (
	"context"

	internalLvmd "github.com/syself/csi-topolvm/internal/lvmd"
	"github.com/syself/csi-topolvm/pkg/lvmd/proto"
	lvmdTypes "github.com/syself/csi-topolvm/pkg/lvmd/types"
)

func NewEmbeddedServiceClients(ctx context.Context, deviceClasses []*lvmdTypes.DeviceClass, LvcreateOptionClasses []*lvmdTypes.LvcreateOptionClass) (
	proto.LVServiceClient,
	proto.VGServiceClient,
) {
	dcManager := internalLvmd.NewDeviceClassManager(deviceClasses)
	lvOptionClassManager := internalLvmd.NewLvcreateOptionClassManager(LvcreateOptionClasses)

	return internalLvmd.NewEmbeddedServiceClients(ctx, dcManager, lvOptionClassManager)
}
