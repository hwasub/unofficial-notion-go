package notionasset

import (
	"net/url"
	"strings"
	"testing"
)

func TestStableSourcesAndCredentials(t *testing.T) {
	attachment := "attachment:file:image.png"
	file := "https://file.notion.com/f/f/space/file/image.png?signature=secret&expirationTimestamp=1"
	cdn := "https://img.notionusercontent.com/s3/prod-files-secure%2Fspace%2Ffile%2Fimage.png/size/w=100?tok=secret"
	for _, input := range []string{attachment, file, cdn, Origin + "/image/" + url.PathEscape(file)} {
		if got := Source(input); got != attachment {
			t.Errorf("Source(%q) = %q", input, got)
		}
		if input != attachment && !Signed(input) {
			t.Errorf("missed signature: %q", input)
		}
	}
	for _, input := range []string{"https://evil.example/image.png?signature=keep", "https://example.com/?token=keep"} {
		if Source(input) != "" || DownloadURL(input) != "" || ForStorage(input) != input {
			t.Errorf("external URL changed: %q", input)
		}
	}
	for _, input := range []string{"https://file.notion.com.evil.example/a", "https://u:p@file.notion.com/a", "http://file.notion.com/a", "https://file.notion.com:444/a", "https://app.notion.com/image/" + url.PathEscape("http://127.0.0.1/private")} {
		if DownloadURL(input) != "" {
			t.Errorf("unsafe download accepted: %q", input)
		}
	}
	opaque := "https://img.notionusercontent.com/opaque?tok=secret"
	if ForStorage(opaque) != "" {
		t.Fatal("opaque signature persisted")
	}
	for i := 0; i < 8; i++ {
		file = Origin + "/image/" + url.PathEscape(file)
	}
	if Source(file) != "" {
		t.Fatal("unbounded proxy nesting")
	}
	legacy := "https://s3.us-west-2.amazonaws.com/secure.notion-static.com/file/a?X-Amz-Signature=secret"
	if strings.Contains(ForStorage(legacy), "?") {
		t.Fatal("legacy signature persisted")
	}
}
