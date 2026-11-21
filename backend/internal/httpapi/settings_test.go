package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSettingsAnswerThePrimaryCurrencyAndTimeZone(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	res := a.do(t, request{path: "/api/settings", cookie: c})

	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)

	var got struct {
		PrimaryCurrency string `json:"primaryCurrency"`
		TimeZone        string `json:"timeZone"`
	}
	if err := json.Unmarshal(res.body, &got); err != nil {
		t.Fatalf("body: %v", err)
	}

	if got.PrimaryCurrency != "RSD" || got.TimeZone != "Europe/Belgrade" {
		t.Errorf("settings = %+v, want RSD and Europe/Belgrade", got)
	}
}
