package controller

const (
	// keySelectedNode is a PVC resource indexing key for the controller
	keySelectedNode = "metadata.annotations.selected-node"

	// keyLogicalVolumeProviderID is a Logical Volume resource indexing key for the controller
	keyLogicalVolumeProviderID = "spec.providerID"

	// AnnSelectedNode annotation is added to a PVC that has been triggered by scheduler to
	// be dynamically provisioned. Its value is the name of the selected node.
	// https://github.com/kubernetes/kubernetes/blob/9bae1bc56804db4905abebcd408e0f02e199ab93/pkg/controller/volume/persistentvolume/util/util.go#L53
	AnnSelectedNode = "volume.kubernetes.io/selected-node"
)
