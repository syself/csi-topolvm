package node

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	topolvm "github.com/syself/csi-topolvm"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func GetNodeByProviderID(ctx context.Context, kubeClient client.Client, providerID string) (*corev1.Node, error) {
	if providerID == "" {
		return nil, fmt.Errorf("GetNodeByProviderID failed. providerID is empty")
	}

	nodeList := &corev1.NodeList{}
	if err := kubeClient.List(ctx, nodeList, &client.MatchingLabels{topolvm.ProviderIDLabel: providerID}); err != nil {
		return nil, fmt.Errorf("GetNodeByProviderID failed to list nodes: %v", err)
	}
	l := len(nodeList.Items)
	if l == 0 {
		// We expect that node has already the appropriate label. The label gets set
		// by the node-driver-registrar.
		return nil, fmt.Errorf("GetNodeByProviderID failed: No node found with label %s=%s",
			topolvm.ProviderIDLabel, providerID)
	}

	if l > 2 {
		return nil, fmt.Errorf("GetNodeByProviderID failed: More than one node found with label %s=%s",
			topolvm.ProviderIDLabel, providerID)
	}
	return &nodeList.Items[0], nil
}

// GetProviderIDByNodeName gets the ProviderID from corresponding label of the node
func GetProviderIDByNodeName(ctx context.Context, kubeClient client.Client, nodeName string) (string, error) {
	kubeNode, err := GetNodeByName(ctx, kubeClient, nodeName)
	if err != nil {
		return "", fmt.Errorf("GetNodeByName failed: %w", err)
	}
	return GetProviderIDByNodeObj(kubeNode)
}

// GetProviderIDByNodeObj gets the ProviderID from corresponding label of the node
func GetProviderIDByNodeObj(kubeNode *corev1.Node) (string, error) {
	labelValue, ok := kubeNode.Labels[topolvm.ProviderIDLabel]
	if ok {
		if labelValue == "" {
			return "", fmt.Errorf("empty label %q Node %q", topolvm.ProviderIDLabel, kubeNode.Name)
		}
		return labelValue, nil
	}
	// Looks like the first volume on that node. The label does not exist yet.
	if kubeNode.Spec.ProviderID == "" {
		return "", fmt.Errorf("label %q not found on Node %q, and spec.providerID empty", topolvm.ProviderIDLabel, kubeNode.Name)
	}
	return sanitizeProviderIDLabelValue(kubeNode.Spec.ProviderID), nil
}

// sanitizeProviderIDLabelValue converts an arbitrary string into a valid Kubernetes label value.
func sanitizeProviderIDLabelValue(input string) string {
	if strings.HasPrefix(input, "hcloud://bm-") {
		input = "hetzner-" + strings.TrimPrefix(input, "hcloud://bm-")
	}
	invalidCharRegex := regexp.MustCompile(`[^a-zA-Z0-9-_.]`)
	transformed := invalidCharRegex.ReplaceAllString(input, "-")
	transformed = regexp.MustCompile(`-+`).ReplaceAllString(transformed, "-")
	transformed = strings.Trim(transformed, "-_.")
	if len(transformed) > 63 {
		hasher := sha256.New()
		hasher.Write([]byte(transformed))
		hashed := hasher.Sum(nil)
		transformed = hex.EncodeToString(hashed)[:63]
	}
	transformed = strings.TrimRight(transformed, "-_.")
	return transformed
}

// GetNodeByName returns the Node by name.
func GetNodeByName(ctx context.Context, kubeClient client.Client, nodeName string) (*corev1.Node, error) {
	nodeObj := &corev1.Node{}
	nn := types.NamespacedName{Name: nodeName}

	logger := log.FromContext(ctx)
	err := retry.OnError(wait.Backoff{
		Steps:    10,
		Duration: 100 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
	},
		func(err error) bool {
			logger.Info("getNodeByName failed, retrying (just info)",
				"err", err,
				"stack", string(debug.Stack()),
			)
			return true
		}, func() error {
			return kubeClient.Get(ctx, nn, nodeObj)
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %q: %w", nodeName, err)
	}
	return nodeObj, nil
}
