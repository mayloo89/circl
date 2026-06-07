package captcha

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTurnstile_Verify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("secret") != "sek" {
			t.Errorf("secret = %q, want sek", r.FormValue("secret"))
		}
		w.Header().Set("Content-Type", "application/json")
		if r.FormValue("response") == "good" {
			_, _ = w.Write([]byte(`{"success":true}`))
		} else {
			_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
		}
	}))
	defer srv.Close()

	v := NewTurnstile("sek")
	v.url = srv.URL

	if ok, err := v.Verify(t.Context(), "good", "1.2.3.4"); err != nil || !ok {
		t.Errorf("Verify(good) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := v.Verify(t.Context(), "bad", ""); err != nil || ok {
		t.Errorf("Verify(bad) = %v, %v; want false, nil", ok, err)
	}
	// An empty token is rejected without hitting the network.
	if ok, err := v.Verify(t.Context(), "", ""); err != nil || ok {
		t.Errorf("Verify(empty) = %v, %v; want false, nil", ok, err)
	}
}
