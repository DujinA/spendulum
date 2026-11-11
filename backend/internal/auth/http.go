package auth

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
	"github.com/DujinA/spendulum/backend/internal/httpapi/problem"
)

const CookieName = "spendulum_session"

const invalidCredentials = "Email or password is incorrect."

type Handler struct {
	svc            *Service
	logger         *slog.Logger
	secureCookie   bool
	trustedProxies []netip.Prefix
}

func NewHandler(svc *Service, logger *slog.Logger, secureCookie bool, trustedProxies []netip.Prefix) Handler {
	return Handler{svc: svc, logger: logger, secureCookie: secureCookie, trustedProxies: trustedProxies}
}

func (h Handler) CreateSession(ctx context.Context, req gen.CreateSessionRequestObject) (gen.CreateSessionResponseObject, error) {
	login, err := h.svc.Login(ctx, req.Body.Email, req.Body.Password, clientIPFrom(ctx))

	var tooMany *TooManyAttemptsError

	switch {
	case errors.As(err, &tooMany):
		h.logger.WarnContext(ctx, "login refused: too many failed attempts")

		return gen.CreateSession429ApplicationProblemPlusJSONResponse{
			Body:    problem.New(http.StatusTooManyRequests, "Too many failed logins. Try again later."),
			Headers: gen.CreateSession429ResponseHeaders{RetryAfter: int(math.Ceil(tooMany.RetryAfter.Seconds()))},
		}, nil
	case errors.Is(err, ErrInvalidCredentials):
		h.logger.InfoContext(ctx, "login failed")

		return gen.CreateSession401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: gen.UnauthorizedApplicationProblemPlusJSONResponse(problem.New(http.StatusUnauthorized, invalidCredentials)),
		}, nil
	case err != nil:
		return nil, err
	}

	h.logger.InfoContext(ctx, "logged in", slog.String("user_id", login.Session.UserID.String()))

	return gen.CreateSession204Response{
		Headers: gen.CreateSession204ResponseHeaders{SetCookie: h.cookie(login.Token, int(SessionLifetime.Seconds())).String()},
	}, nil
}

func (h Handler) GetSession(ctx context.Context, _ gen.GetSessionRequestObject) (gen.GetSessionResponseObject, error) {
	s, _ := SessionFrom(ctx)

	email, err := h.svc.Email(ctx, s.UserID)
	if err != nil {
		return nil, err
	}

	return gen.GetSession200JSONResponse{Email: email}, nil
}

func (h Handler) DeleteSession(ctx context.Context, _ gen.DeleteSessionRequestObject) (gen.DeleteSessionResponseObject, error) {
	s, _ := SessionFrom(ctx)

	if err := h.svc.Logout(ctx, s); err != nil {
		return nil, err
	}

	h.logger.InfoContext(ctx, "logged out", slog.String("user_id", s.UserID.String()))

	return gen.DeleteSession204Response{
		Headers: gen.DeleteSession204ResponseHeaders{SetCookie: h.cookie("", -1).String()},
	}, nil
}

// RequireSession answers 401 without a valid session, except to logging in, and puts the session and client IP in ctx.
func (h Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), clientIPKey{}, h.clientIP(r))

		if r.Method == http.MethodPost && r.URL.Path == "/api/session" {
			next.ServeHTTP(w, r.WithContext(ctx))

			return
		}

		var token string
		if c, err := r.Cookie(CookieName); err == nil {
			token = c.Value
		}

		s, renewed, err := h.svc.Authenticate(ctx, token)

		switch {
		case errors.Is(err, ErrNoSession):
			problem.Write(w, http.StatusUnauthorized, "Log in to continue.")

			return
		case err != nil:
			h.logger.ErrorContext(ctx, "authenticate", slog.Any("error", err))
			problem.Write(w, http.StatusInternalServerError, "")

			return
		}

		if renewed {
			http.SetCookie(w, h.cookie(token, int(SessionLifetime.Seconds())))
		}

		next.ServeHTTP(w, r.WithContext(WithSession(ctx, s)))
	})
}

// Max-Age is always explicit, because iOS home-screen apps drop session cookies; a negative one clears it.
func (h Handler) cookie(token string, maxAge int) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // Secure is off only by configuration, for plain-HTTP test hosts
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	}
}

type clientIPKey struct{}

func clientIPFrom(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)

	return ip
}

// Behind a trusted proxy the client is the last X-Forwarded-For entry, the one the proxy added (ADR 0002).
func (h Handler) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	peer, err := netip.ParseAddr(host)
	if err != nil || !h.trusted(peer.Unmap()) {
		return host
	}

	forwarded := r.Header.Values("X-Forwarded-For")
	if len(forwarded) == 0 {
		return host
	}

	entries := strings.Split(forwarded[len(forwarded)-1], ",")
	if client, err := netip.ParseAddr(strings.TrimSpace(entries[len(entries)-1])); err == nil {
		return client.Unmap().String()
	}

	return host
}

func (h Handler) trusted(addr netip.Addr) bool {
	for _, p := range h.trustedProxies {
		if p.Contains(addr) {
			return true
		}
	}

	return false
}
