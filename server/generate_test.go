package server

import (
	"strings"
	"testing"
)

func TestParseGenerateRequest(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
		wantReq generateRequest
	}{
		{
			name:    "valid with scheme",
			body:    `{"url":"https://example.com","mode":"basic"}`,
			wantReq: generateRequest{URL: "https://example.com", Mode: "basic"},
		},
		{
			name:    "no scheme — prepends https",
			body:    `{"url":"example.com","mode":"basic"}`,
			wantReq: generateRequest{URL: "https://example.com", Mode: "basic"},
		},
		{
			name:    "path and query stripped to origin",
			body:    `{"url":"example.com/docs/page?id=123","mode":"basic"}`,
			wantReq: generateRequest{URL: "https://example.com", Mode: "basic"},
		},
		{
			name:    "valid enhanced with full text",
			body:    `{"url":"https://example.com","mode":"enhanced","full_text":true}`,
			wantReq: generateRequest{URL: "https://example.com", Mode: "enhanced", FullText: true},
		},
		{
			name:    "mode defaults to basic when omitted",
			body:    `{"url":"https://example.com"}`,
			wantReq: generateRequest{URL: "https://example.com", Mode: "basic"},
		},
		{
			name:    "mode defaults to basic when unrecognized",
			body:    `{"url":"https://example.com","mode":"invalid"}`,
			wantReq: generateRequest{URL: "https://example.com", Mode: "basic"},
		},
		{
			name:    "missing url",
			body:    `{"mode":"basic"}`,
			wantErr: true,
		},
		{
			name:    "invalid url",
			body:    `{"url":"://bad","mode":"basic"}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			body:    `{bad json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGenerateRequest(strings.NewReader(tt.body))
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if err != nil {
				return
			}
			if got != tt.wantReq {
				t.Errorf("got %+v, want %+v", got, tt.wantReq)
			}
		})
	}
}
