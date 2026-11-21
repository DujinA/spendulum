package httpapi_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/DujinA/spendulum/backend/internal/fx"
)

func TestCurrenciesListEverySupportedCurrencyWithItsMinorUnits(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	stored := []fx.Currency{{Code: "RSD", MinorUnits: 2}, {Code: "KWD", MinorUnits: 3}, {Code: "JPY", MinorUnits: 0}}
	if err := fx.NewPostgres(a.db.Queries()).SaveCurrencies(t.Context(), stored); err != nil {
		t.Fatalf("SaveCurrencies: %v", err)
	}

	res := a.do(t, request{path: "/api/currencies", cookie: c})

	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)

	type currencyJSON struct {
		Code       string `json:"code"`
		MinorUnits int    `json:"minorUnits"`
	}

	var got []currencyJSON
	if err := json.Unmarshal(res.body, &got); err != nil {
		t.Fatalf("body: %v", err)
	}

	want := []currencyJSON{{"JPY", 0}, {"KWD", 3}, {"RSD", 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("currencies = %+v, want %+v", got, want)
	}
}

func TestCurrenciesNeedASession(t *testing.T) {
	a := newApp(t)

	res := a.do(t, request{path: "/api/currencies"})

	assertStatus(t, res, http.StatusUnauthorized)
	assertMatchesSpec(t, res)
}
