package notionid

import "testing"

func TestParsePageID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"compact", "1ad6e61cf82480c9a6c4d251043457d3", "1ad6e61c-f824-80c9-a6c4-d251043457d3"},
		{"hyphenated", "1AD6E61C-F824-80C9-A6C4-D251043457D3", "1ad6e61c-f824-80c9-a6c4-d251043457d3"},
		{"url last id", "https://example.notion.site/Parent-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/Child-1ad6e61cf82480c9a6c4d251043457d3", "1ad6e61c-f824-80c9-a6c4-d251043457d3"},
		{"invalid", "foo", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParsePageID(tt.in); got != tt.want {
				t.Fatalf("ParsePageID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeBlockID(t *testing.T) {
	if got := NormalizeBlockID("1ad6e61cf82480c9a6c4d251043457d3"); got != "1ad6e61c-f824-80c9-a6c4-d251043457d3" {
		t.Fatalf("NormalizeBlockID compact = %q", got)
	}
	if got := NormalizeBlockID("block:icon"); got != "block:icon" {
		t.Fatalf("NormalizeBlockID fallback = %q", got)
	}
}
