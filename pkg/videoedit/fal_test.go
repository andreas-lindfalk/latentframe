package videoedit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// startFalMock stands up a fake fal queue: any POST is a submit (returns status +
// response urls), /status returns IN_PROGRESS then COMPLETED, /result returns
// resultBody. checkSubmit inspects the submit path + body.
func startFalMock(t *testing.T, resultBody string, checkSubmit func(path string, body map[string]any)) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	var polls int
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Key testkey" {
			t.Errorf("Authorization = %q, want %q", got, "Key testkey")
		}
		switch {
		case r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode submit body: %v", err)
			}
			if checkSubmit != nil {
				checkSubmit(r.URL.Path, body)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"request_id":   "r1",
				"status_url":   srv.URL + "/status",
				"response_url": srv.URL + "/result",
			})
		case r.URL.Path == "/status":
			polls++
			status := "IN_PROGRESS"
			if polls >= 2 {
				status = "COMPLETED"
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
		case r.URL.Path == "/result":
			_, _ = w.Write([]byte(resultBody))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv = httptest.NewServer(mux)
	return srv
}

func testFal(client *http.Client) *Fal {
	return &Fal{apiKey: "testkey", model: ModelKlingO3Pro, http: client, poll: time.Millisecond, timeout: 5 * time.Second}
}

func TestFalTransform(t *testing.T) {
	srv := startFalMock(t, `{"video":{"url":"https://cdn/out.mp4"}}`, func(path string, body map[string]any) {
		if path != "/"+ModelKlingO3Pro {
			t.Errorf("submit path = %q", path)
		}
		if body["video_url"] != "https://in/clip.mp4" || body["prompt"] != "make it lovely" {
			t.Errorf("submit body = %v", body)
		}
	})
	defer srv.Close()
	old := falBase
	falBase = srv.URL
	defer func() { falBase = old }()

	res, err := testFal(srv.Client()).Transform(context.Background(),
		Request{VideoURL: "https://in/clip.mp4", Prompt: "make it lovely"})
	require.NoError(t, err)
	require.Equal(t, "https://cdn/out.mp4", res.VideoURL)
}

func TestFalUpscale(t *testing.T) {
	srv := startFalMock(t, `{"video":{"url":"https://cdn/4k.mp4"}}`, func(path string, body map[string]any) {
		if path != "/"+ModelUpscaleTopaz {
			t.Errorf("submit path = %q, want default upscale model", path)
		}
		if body["video_url"] != "https://in/clip.mp4" {
			t.Errorf("video_url = %v", body["video_url"])
		}
		if body["upscale_factor"] != float64(2) { // Params passed through
			t.Errorf("upscale_factor = %v, want 2", body["upscale_factor"])
		}
	})
	defer srv.Close()
	old := falBase
	falBase = srv.URL
	defer func() { falBase = old }()

	res, err := testFal(srv.Client()).Upscale(context.Background(),
		UpscaleRequest{VideoURL: "https://in/clip.mp4", Params: map[string]any{"upscale_factor": 2}})
	require.NoError(t, err)
	require.Equal(t, "https://cdn/4k.mp4", res.VideoURL)
}

func TestFalValidation(t *testing.T) {
	f := testFal(http.DefaultClient)
	ctx := context.Background()
	require.Error(t, mustErr(f.Transform(ctx, Request{Prompt: "x"})), "Transform without VideoURL")
	require.Error(t, mustErr(f.Transform(ctx, Request{VideoURL: "u"})), "Transform without Prompt")
	require.Error(t, mustErr(f.Upscale(ctx, UpscaleRequest{})), "Upscale without VideoURL")
	require.Error(t, mustErr(f.AddSound(ctx, SoundRequest{})), "AddSound without VideoURL")
}

// mustErr drops the Result so validation cases read as require.Error(...).
func mustErr(_ Result, err error) error { return err }
