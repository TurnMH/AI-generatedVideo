package generators

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestComfyUIVideoGeneratorGenerate(t *testing.T) {
	var uploadedContentType string
	var promptBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/system_stats":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.URL.Path == "/source.png":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("png"))
		case r.URL.Path == "/upload/image":
			uploadedContentType = r.Header.Get("Content-Type")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"uploaded-frame.png","subfolder":"input","type":"input"}`))
		case r.URL.Path == "/prompt":
			body, _ := io.ReadAll(r.Body)
			promptBody = string(body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"prompt_id":"prompt-1"}`))
		case r.URL.Path == "/history/prompt-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"prompt-1":{"status":{"status_str":"success","completed":true},"outputs":{"9":{"gifs":[{"filename":"clip.mp4","subfolder":"videos","type":"output"}]}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	workflow := `{"1":{"inputs":{"image":__IMAGE_JSON__}},"2":{"inputs":{"prompt":__PROMPT_JSON__,"negative":__NEGATIVE_PROMPT_JSON__,"frames":__FRAMES__,"fps":__FPS__,"seed":__SEED__}}}`
	gen := NewComfyUIVideoGenerator(server.URL, workflow)
	if !gen.IsAvailable(context.Background()) {
		t.Fatalf("expected generator to be available")
	}

	clip, err := gen.Generate(context.Background(), VideoGenerateReq{
		SourceImageURL: server.URL + "/source.png",
		Prompt:         "cinematic woman walking to bathroom",
		NegativePrompt: "blurry",
		DurationSec:    5,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if clip.ClipURL != server.URL+"/view?filename=clip.mp4&subfolder=videos&type=output" {
		t.Fatalf("unexpected clip url %q", clip.ClipURL)
	}
	if clip.ModelUsed != "comfyui-video" {
		t.Fatalf("unexpected model used %q", clip.ModelUsed)
	}
	if !strings.Contains(uploadedContentType, "multipart/form-data") {
		t.Fatalf("expected multipart upload, got %q", uploadedContentType)
	}
	for _, want := range []string{`"uploaded-frame.png"`, `cinematic woman walking to bathroom`, `blurry`, `"frames":15`, `"fps":3`} {
		if !strings.Contains(promptBody, want) {
			t.Fatalf("expected prompt body to contain %q, got %s", want, promptBody)
		}
	}
}

func TestComfyUIVideoGeneratorUnavailableWithoutWorkflow(t *testing.T) {
	gen := NewComfyUIVideoGenerator("http://127.0.0.1:8188", "")
	if gen.IsAvailable(context.Background()) {
		t.Fatalf("expected generator without workflow to be unavailable")
	}
}

func TestVideoFramePlanCapsAnimateDiffFrames(t *testing.T) {
	frames, fps := videoFramePlan(5)
	if frames > comfyUIMaxFrames {
		t.Fatalf("expected frames <= %d, got %d", comfyUIMaxFrames, frames)
	}
	if frames != 15 || fps != 3 {
		t.Fatalf("expected capped 5s plan to be 15 frames at 3 fps, got %d frames at %d fps", frames, fps)
	}
}
