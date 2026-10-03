package main

import (
	"bytes"
	"crypto/subtle"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Ahmed-Idani/db_ops"
	"github.com/Ahmed-Idani/stats"
)

//go:embed templates static
var webFS embed.FS

const sessionCookie = "session"

var pages = loadTemplates()

type pageData struct {
	LoggedIn bool
	Error    string
	URL      string
	Name     string
	Created  *db_ops.Link
	Links    []db_ops.Link
	Link     db_ops.Link
	Dash     stats.Dashboard
	Ranges   []stats.Range
	Timezone string
}

func loadTemplates() map[string]*template.Template {
	funcs := template.FuncMap{"statsURL": statsURL, "dict": dict}
	pages := map[string]*template.Template{}
	for _, page := range []string{"login", "links", "stats"} {
		pages[page] = template.Must(template.New("").Funcs(funcs).
			ParseFS(webFS, "templates/layout.html", "templates/"+page+".html"))
	}
	return pages
}

func dict(pairs ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i].(string)] = pairs[i+1]
	}
	return m
}

func statsURL(name, rangeKey string, includeBots bool) string {
	q := url.Values{"range": {rangeKey}}
	if includeBots {
		q.Set("bots", "1")
	}
	return "/admin/links/" + url.PathEscape(name) + "?" + q.Encode()
}

func render(w http.ResponseWriter, status int, page string, data pageData) {
	var buf bytes.Buffer
	if err := pages[page].ExecuteTemplate(&buf, "layout", data); err != nil {
		fmt.Println(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func loggedIn(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(adminToken)) == 1
}

func setSession(w http.ResponseWriter, r *http.Request, path, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

func requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !loggedIn(r) {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func loginPage(w http.ResponseWriter, r *http.Request) {
	if loggedIn(r) {
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
		return
	}
	render(w, http.StatusOK, "login", pageData{})
}

func loginSubmit(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PostFormValue("token"))
	if subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
		render(w, http.StatusUnauthorized, "login", pageData{Error: "Wrong token."})
		return
	}
	setSession(w, r, "/", adminToken, 30*24*60*60)
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func logoutSubmit(w http.ResponseWriter, r *http.Request) {
	setSession(w, r, "/", "", -1)
	setSession(w, r, "/admin", "", -1)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func linksPage(w http.ResponseWriter, r *http.Request) {
	renderLinks(w, http.StatusOK, pageData{}, r.URL.Query().Get("created"))
}

func renderLinks(w http.ResponseWriter, status int, data pageData, created string) {
	links, err := db_ops.ListURLs(db)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data.LoggedIn = true
	data.Links = links
	for i := range links {
		if links[i].Name == created {
			data.Created = &links[i]
		}
	}
	render(w, status, "links", data)
}

func createLinkSubmit(w http.ResponseWriter, r *http.Request) {
	rawURL, name := r.PostFormValue("url"), r.PostFormValue("name")
	link, err := createLink(rawURL, name)
	if err != nil {
		data := pageData{Error: err.Error(), URL: rawURL, Name: name}
		renderLinks(w, createErrorStatus(err), data, "")
		return
	}
	http.Redirect(w, r, "/admin/?created="+link.Name, http.StatusSeeOther)
}

func deleteLinkSubmit(w http.ResponseWriter, r *http.Request) {
	err := db_ops.DeleteURL(db, r.PathValue("name"))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		fmt.Println(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func statsPage(w http.ResponseWriter, r *http.Request) {
	summary, err := db_ops.GetStats(db, r.PathValue("name"))
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		fmt.Println(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	opts := stats.Options{
		Range:       stats.FindRange(r.URL.Query().Get("range")),
		IncludeBots: r.URL.Query().Get("bots") == "1",
		Now:         time.Now(),
		Loc:         timezone,
	}
	clicks, err := db_ops.ListClicks(db, summary.Link.ID, opts.Range.PreviousStart(opts.Now, opts.Loc))
	if err != nil {
		fmt.Println(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	render(w, http.StatusOK, "stats", pageData{
		LoggedIn: true,
		Link:     summary.Link,
		Dash:     stats.Build(clicks, opts),
		Ranges:   stats.Ranges,
		Timezone: timezone.String(),
	})
}

func registerDashboard() {
	http.Handle("GET /admin/static/", http.StripPrefix("/admin", http.FileServerFS(webFS)))
	http.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
	})
	http.HandleFunc("GET /admin/login", loginPage)
	http.HandleFunc("GET /admin/{$}", requireLogin(linksPage))
	http.HandleFunc("GET /admin/links/{name}", requireLogin(statsPage))
}
