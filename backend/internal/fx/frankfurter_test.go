package fx_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/DujinA/spendulum/backend/internal/fx"
)

func TestFrankfurterListsOnlyFiatCurrencies(t *testing.T) {
	srv := stubProvider(t, map[string]string{
		"/v2/currencies": `[
			{"iso_code":"AAVE","iso_numeric":null,"name":"Aave","symbol":"AAVE","type":"crypto","start_date":"2020-10-10","end_date":"2026-11-20"},
			{"iso_code":"EUR","iso_numeric":"978","name":"Euro","symbol":"€","type":"currency","start_date":"1999-01-04","end_date":"2026-11-21"},
			{"iso_code":"RSD","iso_numeric":"941","name":"Serbian Dinar","symbol":"din","type":"currency","start_date":"2000-01-03","end_date":"2026-11-21"},
			{"iso_code":"XAU","iso_numeric":"959","name":"Gold","symbol":null,"type":"metal","start_date":"1968-01-02","end_date":"2026-11-20"}
		]`,
	})

	got, err := fx.NewFrankfurter(srv.URL+"/v2", srv.Client()).Currencies(t.Context())
	if err != nil || !reflect.DeepEqual(got, []string{"EUR", "RSD"}) {
		t.Errorf("Currencies() = %v, %v; want [EUR RSD]", got, err)
	}
}

func TestFrankfurterAsksForADateRangeAgainstEURAndReadsEveryDigit(t *testing.T) {
	var query url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/rates" {
			http.NotFound(w, r)

			return
		}

		query = r.URL.Query()
		_, _ = w.Write([]byte(`[
			{"date":"2026-11-13","base":"EUR","quote":"IRR","rate":2127444},
			{"date":"2026-11-13","base":"EUR","quote":"RSD","rate":117.71},
			{"date":"2026-11-14","base":"EUR","quote":"BTC","rate":0.000017751}
		]`))
	}))
	t.Cleanup(srv.Close)

	got, err := fx.NewFrankfurter(srv.URL+"/v2", srv.Client()).Rates(t.Context(), day(t, "2026-11-13"), day(t, "2026-11-14"))
	if err != nil {
		t.Fatalf("Rates: %v", err)
	}

	if want := (url.Values{"from": {"2026-11-13"}, "to": {"2026-11-14"}, "base": {"EUR"}}); !reflect.DeepEqual(query, want) {
		t.Errorf("query = %v, want %v", query, want)
	}

	assertRates(t, got, []fx.Rate{
		rate(t, "IRR", "2026-11-13", "2127444"),
		rate(t, "RSD", "2026-11-13", "117.71"),
		rate(t, "BTC", "2026-11-14", "0.000017751"),
	})
}

func TestFrankfurterReportsTheProvidersError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"status":422,"message":"invalid currency: XYZ"}`))
	}))
	t.Cleanup(srv.Close)

	_, err := fx.NewFrankfurter(srv.URL+"/v2", srv.Client()).Rates(t.Context(), day(t, "2026-11-13"), day(t, "2026-11-13"))
	if err == nil || !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "invalid currency: XYZ") {
		t.Errorf("Rates error = %v, want the status and the provider's message", err)
	}
}

func TestFrankfurterRejectsARowThatIsNotAPositiveRateAgainstEUR(t *testing.T) {
	for name, row := range map[string]string{
		"another base": `{"date":"2026-11-13","base":"USD","quote":"RSD","rate":107.1}`,
		"zero rate":    `{"date":"2026-11-13","base":"EUR","quote":"RSD","rate":0}`,
		"no date":      `{"base":"EUR","quote":"RSD","rate":117.71}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := stubProvider(t, map[string]string{"/v2/rates": "[" + row + "]"})

			if _, err := fx.NewFrankfurter(srv.URL+"/v2", srv.Client()).Rates(t.Context(), day(t, "2026-11-13"), day(t, "2026-11-13")); err == nil {
				t.Errorf("Rates accepted %s", row)
			}
		})
	}
}

func TestTheE2EStubServesFixedRatesTheFrankfurterAdapterReads(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir("../../../e2e/stubs/rates")))
	t.Cleanup(srv.Close)

	provider := fx.NewFrankfurter(srv.URL+"/v2", srv.Client())

	currencies, err := provider.Currencies(t.Context())
	if err != nil {
		t.Fatalf("Currencies: %v", err)
	}

	for _, code := range []string{"EUR", "JPY", "KWD", "RSD", "USD"} {
		if !slices.Contains(currencies, code) {
			t.Errorf("the stub's currencies %v lack %s", currencies, code)
		}
	}

	rates, err := provider.Rates(t.Context(), day(t, "2030-06-01"), day(t, "2030-06-30"))
	if err != nil || len(rates) == 0 {
		t.Errorf("Rates from the stub = %d rates, %v; want its fixed rates for any range", len(rates), err)
	}
}

// stubProvider answers each path with its fixed body.
func stubProvider(t *testing.T, bodies map[string]string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)

			return
		}

		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return srv
}
