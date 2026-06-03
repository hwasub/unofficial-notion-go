package notion

import "testing"

func TestSafeURLRejectsSmugglingAndUnsafeSchemes(t *testing.T) {
	rejected := []string{
		// dangerous schemes
		"javascript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD4=",
		"vbscript:msgbox(1)",
		// protocol-relative
		"//evil.example.com/phish",
		// backslash smuggling: browsers normalize "\" to "/"
		`/\evil.example.com/phish`,
		`https:\\evil.example.com\phish`,
		`http:\\evil.example.com`,
		// control-character smuggling: browsers strip TAB/CR/LF before parsing
		"/\t/evil.example.com",
		"/\n/evil.example.com",
		"https://good.example.com/\tpath\\x",
		// opaque scheme form with no authority
		"https:evil.example.com",
	}
	for _, in := range rejected {
		if got := safeURL(in); got != "" {
			t.Errorf("safeURL(%q) = %q, want \"\" (should be rejected)", in, got)
		}
	}

	// Legitimate URLs and root-relative paths must still pass unchanged.
	allowed := map[string]string{
		"https://www.example.com/page?q=1": "https://www.example.com/page?q=1",
		"http://example.com":               "http://example.com",
		"/local/asset.png":                 "/local/asset.png",
		"/a/b/c":                           "/a/b/c",
	}
	for in, want := range allowed {
		if got := safeURL(in); got != want {
			t.Errorf("safeURL(%q) = %q, want %q", in, got, want)
		}
	}
}
