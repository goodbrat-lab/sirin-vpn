package engine

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func TestReadBodyEncodings(t *testing.T) {
	const want = `{"ok":true,"turn":"1.2.3.4:3478"}`

	compressGzip := func() []byte {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		_, _ = w.Write([]byte(want))
		_ = w.Close()
		return buf.Bytes()
	}
	compressZlib := func() []byte {
		var buf bytes.Buffer
		w := zlib.NewWriter(&buf)
		_, _ = w.Write([]byte(want))
		_ = w.Close()
		return buf.Bytes()
	}
	compressBrotli := func() []byte {
		var buf bytes.Buffer
		w := brotli.NewWriter(&buf)
		_, _ = w.Write([]byte(want))
		_ = w.Close()
		return buf.Bytes()
	}
	compressZstd := func() []byte {
		var buf bytes.Buffer
		w, err := zstd.NewWriter(&buf)
		if err != nil {
			t.Fatalf("zstd.NewWriter: %v", err)
		}
		_, _ = w.Write([]byte(want))
		_ = w.Close()
		return buf.Bytes()
	}

	cases := []struct {
		name string
		enc  string
		body []byte
	}{
		{"plain", "", []byte(want)},
		{"identity", "identity", []byte(want)},
		{"gzip", "gzip", compressGzip()},
		{"deflate", "deflate", compressZlib()},
		{"br", "br", compressBrotli()},
		{"zstd", "zstd", compressZstd()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{
				Header: http.Header{},
				Body:   io.NopCloser(bytes.NewReader(tc.body)),
			}
			if tc.enc != "" {
				resp.Header.Set("Content-Encoding", tc.enc)
			}
			got, err := readBody(resp)
			if err != nil {
				t.Fatalf("readBody(%q) error: %v", tc.enc, err)
			}
			if string(got) != want {
				t.Fatalf("readBody(%q) = %q, want %q", tc.enc, string(got), want)
			}
		})
	}

	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"lz4"}},
		Body:   io.NopCloser(bytes.NewReader([]byte("junk"))),
	}
	if _, err := readBody(resp); err == nil {
		t.Fatal("expected error for unsupported encoding lz4")
	}
}
