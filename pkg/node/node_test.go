package node

import (
	"strings"
	"testing"
)

func Test_sanitizeProviderIDLabelValue(t *testing.T) {
	for _, tt := range []struct {
		providerID string
		expected   string
	}{
		{"hcloud://bm-2105469", "hetzner-2105469"},
		{"--foo..bar--baz__bu//:ö_", "foo..bar-baz__bu"},
		{strings.Repeat("a", 65), "635361c48bb9eab14198e76ea8ab7f1a41685d6ad62aa9146d301d4f17eb0ae"},
	} {
		actual := sanitizeProviderIDLabelValue(tt.providerID)
		if actual != tt.expected {
			t.Errorf("sanitizeProviderIDLabelValue(%q) -> %q is not good. Expected %q", tt.providerID, actual, tt.expected)
		}
	}
}
