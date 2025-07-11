package console

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/store"
)

const sessionCookie = "rl_session"

// User is the signed-in ops user.
type User struct {
	ID    int64
	Email string
	Name  string
	Role  string
	CSRF  string
	token string
}

type userKey struct{}

func userFrom(ctx context.Context) User {
	u, _ := ctx.Value(userKey{}).(User)
	return u
}

func hashToken(t string) string {
	s := sha256.Sum256([]byte(t))
	return hex.EncodeToString(s[:])
}

func randomToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HashPassword is used by the seed and the admin CLI.
func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h), err
}

func (c *Console) loginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := c.session(r); ok {
		http.Redirect(w, r, "/ops", http.StatusFound)
		return
	}
	c.render(w, r, "login", map[string]any{"Next": safeNext(r.URL.Query().Get("next"))})
}

func (c *Console) login(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	next := safeNext(r.FormValue("next"))
	u, err := c.store.Q.GetOpsUserByEmail(r.Context(), email)
	if err == nil {
		err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(r.FormValue("password")))
	}
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		c.render(w, r, "login", map[string]any{"Error": "Email or password is not right.", "Email": email, "Next": next})
		return
	}
	token := randomToken()
	if err := c.store.Q.CreateSession(r.Context(), db.CreateSessionParams{
		TokenHash: hashToken(token), UserID: u.ID, CsrfToken: randomToken(), ExpiresAt: time.Now().Add(c.cfg.SessionTTL),
	}); err != nil {
		c.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/ops", HttpOnly: true, Secure: c.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode, MaxAge: int(c.cfg.SessionTTL.Seconds()),
	})
	c.log.Info("console login", "email", u.Email)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (c *Console) logout(w http.ResponseWriter, r *http.Request) {
	if u, ok := c.session(r); ok {
		_ = c.store.Q.DeleteSession(r.Context(), hashToken(u.token))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (c *Console) session(r *http.Request) (User, bool) {
	ck, err := r.Cookie(sessionCookie)
	if err != nil || ck.Value == "" {
		return User{}, false
	}
	s, err := c.store.Q.GetSession(r.Context(), hashToken(ck.Value))
	if err != nil {
		if !store.IsNotFound(err) {
			c.log.Error("load session", "err", err)
		}
		return User{}, false
	}
	return User{ID: s.UserID, Email: s.Email, Name: s.Name, Role: s.Role, CSRF: s.CsrfToken, token: ck.Value}, true
}

func (c *Console) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := c.session(r)
		if !ok {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	})
}

func (c *Console) checkCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			u := userFrom(r.Context())
			if subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(u.CSRF)) != 1 {
				http.Error(w, "This form has expired. Go back, refresh the page and try again.", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func safeNext(next string) string {
	if strings.HasPrefix(next, "/ops") && !strings.HasPrefix(next, "//") {
		return next
	}
	return "/ops"
}
