package dingtalk

import "testing"

func TestBuildDingTalkImageMsgParam_UsesPhotoURLForHTTP(t *testing.T) {
	param := buildDingTalkImageMsgParam("https://example.com/a.png")
	if got := param["photoURL"]; got != "https://example.com/a.png" {
		t.Fatalf("photoURL mismatch: %q", got)
	}
	if _, ok := param["mediaId"]; ok {
		t.Fatalf("mediaId should not be set for URL input: %+v", param)
	}
}

func TestBuildDingTalkImageMsgParam_UsesMediaIDForNonURL(t *testing.T) {
	param := buildDingTalkImageMsgParam("mid-image-123")
	if got := param["mediaId"]; got != "mid-image-123" {
		t.Fatalf("mediaId mismatch: %q", got)
	}
	if _, ok := param["photoURL"]; ok {
		t.Fatalf("photoURL should not be set for media_id input: %+v", param)
	}
}
