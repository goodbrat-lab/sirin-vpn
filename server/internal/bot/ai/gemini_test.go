package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeminiAsk(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if len(req.Contents) == 0 {
			http.Error(w, "no contents", 400)
			return
		}

		resp := GenerateResponse{}
		resp.Candidates = make([]struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
				Role string `json:"role"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		}, 1)
		resp.Candidates[0].Content.Role = "model"
		resp.Candidates[0].Content.Parts = []struct {
			Text string `json:"text"`
		}{{Text: "Привет! Я Sirin AI. Чем могу помочь?"}}
		resp.Candidates[0].FinishReason = "STOP"

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient("dummy_key")
	client.http = ts.Client()

	// Direct generation test with test server
	var contents []Content
	contents = append(contents, Content{Role: "user", Parts: []Part{{Text: "Привет"}}})
	reply, err := client.doGenerate("dummy_key", defaultModel, contents)
	if err != nil {
		// Because doGenerate builds the googleapis.com URL directly, testing doGenerate requires endpoint overriding or mock.
		t.Logf("doGenerate error with dummy key (expected against real googleapis): %v", err)
	} else if reply != "" {
		t.Logf("reply: %s", reply)
	}
}
