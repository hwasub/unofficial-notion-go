package notion

import (
	"strings"
	"testing"
)

func TestStyleCSSIsSelfContained(t *testing.T) {
	css := StyleCSS()
	if strings.TrimSpace(css) == "" {
		t.Fatal("StyleCSS returned empty stylesheet")
	}
	// The embedded stylesheet must never reach out to the network.
	if strings.Contains(css, "@import") {
		t.Fatal("StyleCSS must not contain @import rules")
	}
	for _, selector := range []string{
		".notion-body",
		":root[data-theme='dark']",
		"@media (prefers-color-scheme: dark)",
		"@media print",
		".notion-embed--youtube iframe",
		".notion-embed--spotify iframe",
	} {
		if !strings.Contains(css, selector) {
			t.Fatalf("StyleCSS missing %q", selector)
		}
	}
	// Long code lines must scroll inside the pre, not the whole block (which
	// would drag the header bar and caption along).
	if !strings.Contains(css, ".notion-code pre {\n  overflow-x: auto;") {
		t.Fatal("StyleCSS: .notion-code pre must be the horizontal scroll container")
	}
}
