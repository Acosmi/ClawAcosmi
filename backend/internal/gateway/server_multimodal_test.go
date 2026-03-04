package gateway

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/channels"
)

type fakeFeishuDownloader struct {
	images map[string][]byte
	files  map[string][]byte

	imageErr map[string]error
	fileErr  map[string]error

	imageCalls []string
	fileCalls  []string
}

func (f *fakeFeishuDownloader) DownloadImage(_ context.Context, _ string, imageKey string) ([]byte, error) {
	f.imageCalls = append(f.imageCalls, imageKey)
	if err := f.imageErr[imageKey]; err != nil {
		return nil, err
	}
	if data, ok := f.images[imageKey]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("image %s not found", imageKey)
}

func (f *fakeFeishuDownloader) DownloadFile(_ context.Context, _ string, fileKey string) ([]byte, error) {
	f.fileCalls = append(f.fileCalls, fileKey)
	if err := f.fileErr[fileKey]; err != nil {
		return nil, err
	}
	if data, ok := f.files[fileKey]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("file %s not found", fileKey)
}

type fakeMMSTTProvider struct {
	transcript string
	err        error
}

func (f *fakeMMSTTProvider) Transcribe(_ context.Context, _ []byte, _ string) (string, error) {
	return f.transcript, f.err
}

func (f *fakeMMSTTProvider) Name() string { return "fake-stt" }

func (f *fakeMMSTTProvider) TestConnection(_ context.Context) error { return nil }

type fakeMMDocConverter struct {
	markdown string
	err      error
}

func (f *fakeMMDocConverter) Convert(_ context.Context, _ []byte, _, _ string) (string, error) {
	return f.markdown, f.err
}

func (f *fakeMMDocConverter) SupportedFormats() []string { return []string{".pdf"} }

func (f *fakeMMDocConverter) Name() string { return "fake-docconv" }

func (f *fakeMMDocConverter) TestConnection(_ context.Context) error { return nil }

type fakeImageDescriber struct {
	description string
	err         error
}

func (f *fakeImageDescriber) Describe(_ context.Context, _ []byte, _ string) (string, error) {
	return f.description, f.err
}

func (f *fakeImageDescriber) Name() string { return "fake-image" }

func (f *fakeImageDescriber) TestConnection(_ context.Context) error { return nil }

func TestProcessFeishuMessage_MultiImageKeepsAllAndLegacyFirst(t *testing.T) {
	t.Parallel()

	png := []byte{0x89, 0x50, 0x4E, 0x47}
	jpg := []byte{0xFF, 0xD8, 0xFF, 0x11}
	downloader := &fakeFeishuDownloader{
		images: map[string][]byte{
			"img-1": png,
			"img-2": jpg,
		},
	}
	processor := &MultimodalPreprocessor{}
	msg := &channels.ChannelMessage{
		Text:      "hello",
		MessageID: "m-1",
		Attachments: []channels.ChannelAttachment{
			{Category: "image", FileKey: "img-1"},
			{Category: "image", FileKey: "img-2", MimeType: "image/jpeg"},
		},
	}

	result := processor.processFeishuMessageWithDownloader(context.Background(), downloader, msg)

	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(result.Images))
	}
	wantFirst := base64.StdEncoding.EncodeToString(png)
	wantSecond := base64.StdEncoding.EncodeToString(jpg)
	if result.Images[0].Base64 != wantFirst || result.Images[0].MimeType != "image/png" {
		t.Fatalf("unexpected first image: %+v", result.Images[0])
	}
	if result.Images[1].Base64 != wantSecond || result.Images[1].MimeType != "image/jpeg" {
		t.Fatalf("unexpected second image: %+v", result.Images[1])
	}
	if result.ImageBase64 != wantFirst || result.ImageMimeType != "image/png" {
		t.Fatalf("legacy fields should keep first image, got base64=%q mime=%q", result.ImageBase64, result.ImageMimeType)
	}
	if !strings.Contains(result.Text, "hello") {
		t.Fatalf("result text should keep original text, got: %s", result.Text)
	}
	firstImageIdx := strings.Index(result.Text, "[图片: image/png")
	secondImageIdx := strings.Index(result.Text, "[图片: image/jpeg")
	if firstImageIdx < 0 || secondImageIdx < 0 || firstImageIdx > secondImageIdx {
		t.Fatalf("image description order mismatch, text=%s", result.Text)
	}
}

func TestProcessFeishuMessage_TruncatesAttachmentCountToTen(t *testing.T) {
	t.Parallel()

	attachments := make([]channels.ChannelAttachment, 0, 12)
	for i := 0; i < 12; i++ {
		attachments = append(attachments, channels.ChannelAttachment{
			Category: "custom",
			FileName: fmt.Sprintf("att-%d.bin", i),
		})
	}

	processor := &MultimodalPreprocessor{}
	result := processor.processFeishuMessageWithDownloader(context.Background(), &fakeFeishuDownloader{}, &channels.ChannelMessage{
		Text:        "base",
		MessageID:   "m-2",
		Attachments: attachments,
	})

	if result == nil {
		t.Fatal("result should not be nil")
	}
	if got := strings.Count(result.Text, "[附件:"); got != 10 {
		t.Fatalf("expected 10 attachment entries, got %d text=%s", got, result.Text)
	}
	if strings.Contains(result.Text, "att-10.bin") || strings.Contains(result.Text, "att-11.bin") {
		t.Fatalf("attachments beyond limit should be truncated, got text=%s", result.Text)
	}
}

func TestProcessFeishuMessage_AudioAndDocumentPreprocess(t *testing.T) {
	t.Parallel()

	downloader := &fakeFeishuDownloader{
		files: map[string][]byte{
			"aud-1": []byte{0x01, 0x02},
			"doc-1": []byte("%PDF-1.4"),
		},
	}
	processor := &MultimodalPreprocessor{
		STTProvider:  &fakeMMSTTProvider{transcript: "你好，世界"},
		DocConverter: &fakeMMDocConverter{markdown: "# 文档正文"},
	}
	msg := &channels.ChannelMessage{
		Text:      "base",
		MessageID: "m-3",
		Attachments: []channels.ChannelAttachment{
			{Category: "audio", FileKey: "aud-1", MimeType: "audio/opus"},
			{Category: "document", FileKey: "doc-1", FileName: "demo.pdf", MimeType: "application/pdf"},
		},
	}

	result := processor.processFeishuMessageWithDownloader(context.Background(), downloader, msg)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !strings.Contains(result.Text, "[语音转录]: 你好，世界") {
		t.Fatalf("audio transcription missing, text=%s", result.Text)
	}
	if !strings.Contains(result.Text, "[文件: demo.pdf]\n# 文档正文") {
		t.Fatalf("document conversion missing, text=%s", result.Text)
	}
}

func TestProcessFeishuMessage_UsesImageDescriberWhenConfigured(t *testing.T) {
	t.Parallel()

	processor := &MultimodalPreprocessor{
		ImageDescriber: &fakeImageDescriber{description: "一张产品界面截图"},
	}
	downloader := &fakeFeishuDownloader{
		images: map[string][]byte{
			"img-3": []byte{0x89, 0x50, 0x4E, 0x47},
		},
	}
	msg := &channels.ChannelMessage{
		MessageID: "m-4",
		Attachments: []channels.ChannelAttachment{
			{Category: "image", FileKey: "img-3"},
		},
	}

	result := processor.processFeishuMessageWithDownloader(context.Background(), downloader, msg)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !strings.Contains(result.Text, "[图片描述]: 一张产品界面截图") {
		t.Fatalf("image description missing, text=%s", result.Text)
	}
	if len(result.Images) != 1 {
		t.Fatalf("expected one image payload, got %d", len(result.Images))
	}
}

func TestProcessGenericChannelMessage_DataURLImageAndAudio(t *testing.T) {
	t.Parallel()

	png := []byte{0x89, 0x50, 0x4E, 0x47}
	audio := []byte{0x01, 0x02, 0x03}
	processor := &MultimodalPreprocessor{
		STTProvider: &fakeMMSTTProvider{transcript: "测试语音"},
	}
	msg := &channels.ChannelMessage{
		Text: "base",
		Attachments: []channels.ChannelAttachment{
			{
				Category: "image",
				DataURL:  "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
			},
			{
				Category: "audio",
				DataURL:  "data:audio/ogg;base64," + base64.StdEncoding.EncodeToString(audio),
			},
		},
	}

	result := processor.ProcessGenericChannelMessage(context.Background(), msg)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !strings.Contains(result.Text, "base") {
		t.Fatalf("result should keep base text, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "[语音转录]: 测试语音") {
		t.Fatalf("audio transcript missing: %s", result.Text)
	}
	if len(result.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(result.Images))
	}
	wantImage := base64.StdEncoding.EncodeToString(png)
	if result.Images[0].Base64 != wantImage || result.Images[0].MimeType != "image/png" {
		t.Fatalf("unexpected image payload: %+v", result.Images[0])
	}
	if result.ImageBase64 != wantImage || result.ImageMimeType != "image/png" {
		t.Fatalf("legacy image fields mismatch: base64=%q mime=%q", result.ImageBase64, result.ImageMimeType)
	}
}

func TestProcessGenericChannelMessage_DocumentConvertFromDataURL(t *testing.T) {
	t.Parallel()

	processor := &MultimodalPreprocessor{
		DocConverter: &fakeMMDocConverter{markdown: "# 转换成功"},
	}
	msg := &channels.ChannelMessage{
		Attachments: []channels.ChannelAttachment{
			{
				Category: "document",
				FileName: "demo.pdf",
				DataURL:  "data:application/pdf;base64," + base64.StdEncoding.EncodeToString([]byte("%PDF-1.4")),
			},
		},
	}

	result := processor.ProcessGenericChannelMessage(context.Background(), msg)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !strings.Contains(result.Text, "[文件: demo.pdf]\n# 转换成功") {
		t.Fatalf("document conversion missing: %s", result.Text)
	}
}
