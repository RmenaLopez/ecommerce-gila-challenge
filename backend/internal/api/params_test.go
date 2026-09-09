package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestParseIntParam(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		wantN  int
		wantOK bool
	}{
		{"empty defaults to zero", "", 0, true},
		{"valid number", "42", 42, true},
		{"invalid number", "abc", 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			query := url.Values{}
			if c.raw != "" {
				query.Set("limit", c.raw)
			}
			w := httptest.NewRecorder()

			n, ok := parseIntParam(w, query, "limit")

			if n != c.wantN {
				t.Errorf("n = %d, want %d", n, c.wantN)
			}
			if ok != c.wantOK {
				t.Errorf("ok = %v, want %v", ok, c.wantOK)
			}
			if !c.wantOK && w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}
