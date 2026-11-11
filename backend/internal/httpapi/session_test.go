package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLoginSetsA30DayHttpOnlySecureLaxSessionCookie(t *testing.T) {
	a := newApp(t)
	a.account(t)

	res := a.do(t, request{method: http.MethodPost, path: "/api/session", json: credentials("Ana@Example.test", password)})

	assertStatus(t, res, http.StatusNoContent)
	assertMatchesSpec(t, res)

	c := sessionCookie(t, res)
	if c.MaxAge != 30*24*60*60 || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Value == "" {
		t.Errorf("cookie = %+v, want Max-Age 2592000, HttpOnly, Secure, SameSite=Lax, Path=/", c)
	}

	if got := a.do(t, request{path: "/api/session", cookie: c}); got.StatusCode != http.StatusOK || !strings.Contains(string(got.body), `"email":"ana@example.test"`) {
		t.Errorf("GET /api/session with the cookie = %d %s, want 200 with the email", got.StatusCode, got.body)
	}
}

func TestWrongPasswordAndUnknownEmailAnswerTheSame401(t *testing.T) {
	a := newApp(t)
	a.account(t)

	wrong := a.do(t, request{method: http.MethodPost, path: "/api/session", json: credentials(email, "wrong password!")})
	unknown := a.do(t, request{method: http.MethodPost, path: "/api/session", json: credentials("bob@example.test", password)})

	for name, res := range map[string]response{"wrong password": wrong, "unknown email": unknown} {
		assertStatus(t, res, http.StatusUnauthorized)
		assertMatchesSpec(t, res)
		assertProblem(t, res, http.StatusUnauthorized)

		if !strings.Contains(string(res.body), "Email or password is incorrect") {
			t.Errorf("%s: body = %s, want \"Email or password is incorrect\"", name, res.body)
		}

		if len(res.Cookies()) != 0 {
			t.Errorf("%s: a cookie was set: %v", name, res.Cookies())
		}
	}

	if string(wrong.body) != string(unknown.body) {
		t.Errorf("bodies differ:\nwrong password: %s\nunknown email:  %s", wrong.body, unknown.body)
	}
}

func TestSixthLoginWithin15MinutesAfterFiveFailuresAnswers429WithRetryAfter(t *testing.T) {
	a := newApp(t)
	a.account(t)

	for i := range 5 {
		res := a.do(t, request{method: http.MethodPost, path: "/api/session", json: credentials(email, "wrong password!")})
		assertStatus(t, res, http.StatusUnauthorized)

		if i < 4 {
			a.advance(time.Minute)
		}
	}

	res := a.do(t, request{method: http.MethodPost, path: "/api/session", json: credentials(email, password)})

	assertStatus(t, res, http.StatusTooManyRequests)
	assertMatchesSpec(t, res)
	assertProblem(t, res, http.StatusTooManyRequests)

	// The first failure was 4 minutes ago, so the wait is 11 minutes.
	if got := res.Header.Get("Retry-After"); got != "660" {
		t.Errorf("Retry-After = %q, want 660", got)
	}
}

func TestFailedLoginsAreCountedPerClientIPBehindTheTrustedProxy(t *testing.T) {
	a := newApp(t)
	a.account(t)

	viaCaddy := func(client string) request {
		return request{
			method: http.MethodPost, path: "/api/session", json: credentials(email, "wrong password!"),
			remoteAddr: "10.42.0.1:51000", header: map[string]string{"X-Forwarded-For": client},
		}
	}

	for range 5 {
		a.do(t, viaCaddy("203.0.113.9"))
	}

	if res := a.do(t, viaCaddy("203.0.113.9")); res.StatusCode != http.StatusTooManyRequests {
		t.Errorf("6th attempt from the same client = %d, want 429", res.StatusCode)
	}

	if res := a.do(t, viaCaddy("198.51.100.7")); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("attempt from another client = %d, want 401", res.StatusCode)
	}

	// An untrusted peer cannot pick its IP with X-Forwarded-For.
	spoofed := viaCaddy("192.0.2.200")
	spoofed.remoteAddr = "192.0.2.50:4000"

	for range 5 {
		a.do(t, spoofed)
	}

	spoofed.header["X-Forwarded-For"] = "192.0.2.201"
	if res := a.do(t, spoofed); res.StatusCode != http.StatusTooManyRequests {
		t.Errorf("untrusted peer with a new X-Forwarded-For = %d, want 429", res.StatusCode)
	}
}

func TestSessionOlderThan15DaysIsRenewedTo30DaysFromNow(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	a.advance(10 * 24 * time.Hour)

	if res := a.do(t, request{path: "/api/session", cookie: c}); len(res.Cookies()) != 0 {
		t.Errorf("a 10-day-old session was renewed: %v", res.Cookies())
	}

	a.advance(6 * 24 * time.Hour)

	res := a.do(t, request{path: "/api/session", cookie: c})
	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)

	renewed := sessionCookie(t, res)
	if renewed.Value != c.Value || renewed.MaxAge != 30*24*60*60 || !renewed.HttpOnly || !renewed.Secure {
		t.Errorf("renewed cookie = %+v, want the same token with Max-Age 2592000", renewed)
	}

	// Without the renewal the session would end 30 days after login, 14 days from now.
	a.advance(20 * 24 * time.Hour)

	if res := a.do(t, request{path: "/api/session", cookie: c}); res.StatusCode != http.StatusOK {
		t.Errorf("36 days after login and 20 after renewal: status %d, want 200", res.StatusCode)
	}
}

func TestLogoutDeletesTheSessionAndTheOldCookieAnswers401(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	res := a.do(t, request{method: http.MethodDelete, path: "/api/session", cookie: c})
	assertStatus(t, res, http.StatusNoContent)
	assertMatchesSpec(t, res)

	if cleared := sessionCookie(t, res); cleared.MaxAge >= 0 || cleared.Value != "" {
		t.Errorf("logout cookie = %+v, want it cleared", cleared)
	}

	old := a.do(t, request{path: "/api/session", cookie: c})
	assertStatus(t, old, http.StatusUnauthorized)
	assertProblem(t, old, http.StatusUnauthorized)
}

func TestAPIRequestWithoutAValidSessionAnswers401ProblemJSON(t *testing.T) {
	a := newApp(t)
	a.account(t)

	bogus := &http.Cookie{Name: "spendulum_session", Value: "not-a-session"}

	for _, r := range []request{
		{path: "/api/session"},
		{path: "/api/session", cookie: bogus},
		{method: http.MethodDelete, path: "/api/session"},
		{path: "/api/entries"},
		{path: "/api"},
		{method: http.MethodPost, path: "/api/entries", json: "{}"},
	} {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			res := a.do(t, r)

			assertStatus(t, res, http.StatusUnauthorized)
			assertProblem(t, res, http.StatusUnauthorized)

			if got := res.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
		})
	}
}

func TestMutatingAPIRequestWithoutJSONContentTypeAnswers415(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	for _, r := range []request{
		{method: http.MethodPost, path: "/api/session", json: credentials(email, password), contentType: "application/x-www-form-urlencoded"},
		{method: http.MethodPost, path: "/api/session", json: credentials(email, password), contentType: "text/plain"},
		{method: http.MethodDelete, path: "/api/session", cookie: c, contentType: "multipart/form-data"},
	} {
		t.Run(r.method+" "+r.contentType, func(t *testing.T) {
			res := a.do(t, r)

			assertStatus(t, res, http.StatusUnsupportedMediaType)
			assertProblem(t, res, http.StatusUnsupportedMediaType)
		})
	}

	res := a.do(t, request{method: http.MethodPost, path: "/api/session", json: credentials(email, password), contentType: "application/json; charset=utf-8"})
	assertStatus(t, res, http.StatusNoContent)
}

func TestCrossOriginMutatingAPIRequestAnswers403(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	for name, header := range map[string]map[string]string{
		"Sec-Fetch-Site cross-site": {"Sec-Fetch-Site": "cross-site"},
		"foreign Origin":            {"Origin": "https://evil.example"},
	} {
		t.Run(name, func(t *testing.T) {
			res := a.do(t, request{method: http.MethodDelete, path: "/api/session", cookie: c, header: header})

			assertStatus(t, res, http.StatusForbidden)
			assertProblem(t, res, http.StatusForbidden)
		})
	}

	if res := a.do(t, request{path: "/api/session", cookie: c}); res.StatusCode != http.StatusOK {
		t.Errorf("the session was deleted by a refused request: GET /api/session = %d", res.StatusCode)
	}

	same := a.do(t, request{method: http.MethodDelete, path: "/api/session", cookie: c, header: map[string]string{"Sec-Fetch-Site": "same-origin"}})
	assertStatus(t, same, http.StatusNoContent)
}

func TestLoginLogoutAndFailureLogsNeverContainTheEmailOrPassword(t *testing.T) {
	a := newApp(t)
	a.account(t)

	for range 6 {
		a.do(t, request{method: http.MethodPost, path: "/api/session", json: credentials("Ana@Example.test", "wrong password!"), remoteAddr: "192.0.2.9:1234"})
	}

	a.do(t, request{method: http.MethodPost, path: "/api/session", json: credentials("bob@example.test", password)})

	c := a.login(t)
	a.do(t, request{method: http.MethodDelete, path: "/api/session", cookie: c})

	logs := a.logs.String()
	for _, secret := range []string{"ana@example.test", "Ana@Example.test", "bob@example.test", "wrong password!", password, c.Value} {
		if strings.Contains(strings.ToLower(logs), strings.ToLower(secret)) {
			t.Errorf("logs contain %q:\n%s", secret, logs)
		}
	}

	userIDs := map[string]bool{}

	for line := range strings.Lines(logs) {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line is not JSON: %s", line)
		}

		if id, ok := entry["user_id"].(string); ok && (entry["msg"] == "logged in" || entry["msg"] == "logged out") {
			userIDs[id] = true
		}
	}

	if len(userIDs) != 1 {
		t.Errorf("login and logout logged user ids %v, want one user_id on both", userIDs)
	}
}
