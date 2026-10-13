package console

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/ledger"
	"github.com/remitlane/remitlane/internal/money"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

var pageNames = []string{
	"login", "dashboard", "payouts", "payout", "partners", "partner", "ledger",
	"webhooks", "limits", "recon", "providers", "sandbox", "error",
}

func (c *Console) parse() (map[string]*template.Template, error) {
	pages := map[string]*template.Template{}
	for _, name := range pageNames {
		t, err := template.New("layout.html").Funcs(c.funcs()).ParseFS(templateFS, "templates/layout.html", "templates/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("console: parse %s: %w", name, err)
		}
		pages[name] = t
	}
	return pages, nil
}

func (c *Console) funcs() template.FuncMap {
	return template.FuncMap{
		"amount": formatAmount,
		"money":  func(minor int64, cur string) string { return money.Format(minor, money.Currency(cur)) },
		"when":   c.clock.Display,
		"whenp": func(t pgtype.Timestamptz) string {
			if !t.Valid {
				return ""
			}
			return c.clock.Display(t.Time)
		},
		"ago":      ago,
		"date":     clock.DisplayDate,
		"account":  ledger.ShortCode,
		"label":    label,
		"provider": providerName,
		"lower":    strings.ToLower,
		"add":      func(a, b int) int { return a + b },
		"pct":      pct,
		"seconds":  func(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) + "s" },
		"pageLink": pageLink,
	}
}

// formatAmount renders minor units with two decimals and thousands
// separators: 1234567 NGN is "NGN 12,345.67".
func formatAmount(minor int64, currency string) string {
	return currency + " " + money.Group(strconv.FormatFloat(float64(minor)/100, 'f', 2, 64))
}

var providerNames = map[string]string{"swiftpay": "Swiftpay", "momobridge": "MoMoBridge"}

func providerName(s string) string {
	if n, ok := providerNames[s]; ok {
		return n
	}
	return label(s)
}

func label(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func pct(part, total int) string {
	if total == 0 {
		return "0%"
	}
	return strconv.Itoa(part*100/total) + "%"
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}

// pageLink keeps the current query string and swaps the page number.
func pageLink(q url.Values, page int) string {
	v := url.Values{}
	for k, vals := range q {
		v[k] = append([]string(nil), vals...)
	}
	v.Set("page", strconv.Itoa(page))
	return "?" + v.Encode()
}

type flash struct {
	Kind string
	Text string
}

const flashCookie = "rl_flash"

func setFlash(w http.ResponseWriter, kind, text string) {
	http.SetCookie(w, &http.Cookie{Name: flashCookie, Value: url.QueryEscape(kind + "|" + text), Path: "/ops", HttpOnly: true, MaxAge: 60})
}

func popFlash(w http.ResponseWriter, r *http.Request) *flash {
	ck, err := r.Cookie(flashCookie)
	if err != nil {
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: flashCookie, Value: "", Path: "/ops", MaxAge: -1})
	v, _ := url.QueryUnescape(ck.Value)
	kind, text, ok := strings.Cut(v, "|")
	if !ok {
		return nil
	}
	return &flash{Kind: kind, Text: text}
}

type page struct {
	Title    string
	Section  string
	User     User
	Flash    *flash
	DevTools bool
	Env      string
	Today    string
	Data     any
}

func (c *Console) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	c.renderTitled(w, r, name, "", data)
}

func (c *Console) renderTitled(w http.ResponseWriter, r *http.Request, name, title string, data any) {
	t, ok := c.pages[name]
	if !ok {
		http.Error(w, "unknown page "+name, http.StatusInternalServerError)
		return
	}
	p := page{
		Title: title, Section: name, User: userFrom(r.Context()), Flash: popFlash(w, r),
		DevTools: c.cfg.DevTools, Env: c.cfg.Env, Today: c.clock.Today(), Data: data,
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.html", p); err != nil {
		c.log.Error("render", "page", name, "err", err)
		http.Error(w, "could not render page", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (c *Console) fail(w http.ResponseWriter, r *http.Request, err error) {
	c.log.Error("console error", "path", r.URL.Path, "err", err)
	w.WriteHeader(http.StatusInternalServerError)
	c.renderTitled(w, r, "error", "Something went wrong", map[string]string{"Message": "Something went wrong loading this page. It has been logged."})
}

func (c *Console) notFound(w http.ResponseWriter, r *http.Request, what string) {
	w.WriteHeader(http.StatusNotFound)
	c.renderTitled(w, r, "error", "Not found", map[string]string{"Message": "No such " + what + "."})
}
