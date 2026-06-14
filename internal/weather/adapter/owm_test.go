package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleResponse = `{
	"weather": [{"icon": "10d", "description": "rain"}],
	"main": {"temp": 12.5, "humidity": 80}
}`

func TestFetchSuccess(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(sampleResponse))
	}))
	defer srv.Close()

	o := NewOWM("secret-key", 47.36, 19.45, "")
	o.baseURL = srv.URL

	got, err := o.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.IconCode != "10d" || got.Temp != 12.5 || got.Humidity != 80 {
		t.Errorf("unexpected payload: %+v", got)
	}

	for _, want := range []string{"lat=47.36", "lon=19.45", "appid=secret-key", "units=metric"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestFetchHonorsUnits(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(sampleResponse))
	}))
	defer srv.Close()

	o := NewOWM("k", 0, 0, "imperial")
	o.baseURL = srv.URL

	if _, err := o.Fetch(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "units=imperial") {
		t.Errorf("query %q missing units=imperial", gotQuery)
	}
}

func TestFetchNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	o := NewOWM("bad-key", 0, 0, "metric")
	o.baseURL = srv.URL

	_, err := o.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error %q should mention status 401", err)
	}
}

func TestFetchRequestError(t *testing.T) {
	o := NewOWM("k", 0, 0, "metric")
	o.baseURL = "http://127.0.0.1:0" // nothing listening

	if _, err := o.Fetch(context.Background()); err == nil {
		t.Fatal("expected error for unreachable host")
	}
}

func TestParseOWM(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"valid", sampleResponse, false},
		{"missing weather", `{"main": {"temp": 5}}`, true},
		{"malformed json", `{not json`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseOWM(strings.NewReader(tc.body))
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
