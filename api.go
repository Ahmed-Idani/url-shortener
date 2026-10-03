package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Ahmed-Idani/db_ops"
)

const slugChars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var validName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func randomSlug() string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = slugChars[rand.IntN(len(slugChars))]
	}
	return string(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func hasToken(r *http.Request) bool {
	got := []byte(r.Header.Get("Authorization"))
	want := []byte("Bearer " + adminToken)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// fromBrowser reports whether the request comes from the dashboard (login cookie)
// rather than an API client (bearer token).
func fromBrowser(r *http.Request) bool {
	return r.Header.Get("Authorization") == ""
}

func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasToken(r) || (fromBrowser(r) && loggedIn(r)) {
			next.ServeHTTP(w, r)
			return
		}
		if fromBrowser(r) && strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
	})
}

func createLinkHandler(w http.ResponseWriter, r *http.Request) {
	if fromBrowser(r) {
		createLinkSubmit(w, r)
		return
	}
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	link, err := createLink(req.URL, req.Name)
	if err != nil {
		writeError(w, createErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

var (
	errInvalidURL  = errors.New("url must be a valid http or https url")
	errInvalidName = errors.New("name must be 1-64 letters, digits, - or _")
	errInternal    = errors.New("internal error")
)

var reservedNames = map[string]bool{"admin": true, "api": true}

func createLink(rawURL, name string) (db_ops.Link, error) {
	target, err := url.ParseRequestURI(rawURL)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return db_ops.Link{}, errInvalidURL
	}
	custom := name != ""
	if custom && !validName.MatchString(name) {
		return db_ops.Link{}, errInvalidName
	}
	if reservedNames[name] {
		return db_ops.Link{}, db_ops.ErrNameTaken
	}

	for attempt := 0; ; attempt++ {
		if !custom {
			name = randomSlug()
		}
		err = db_ops.CreateURL(db, name, rawURL, baseURL+"/"+name)
		if !errors.Is(err, db_ops.ErrNameTaken) || custom || attempt == 4 {
			break
		}
	}
	if errors.Is(err, db_ops.ErrNameTaken) {
		return db_ops.Link{}, err
	}
	if err != nil {
		fmt.Println(err)
		return db_ops.Link{}, errInternal
	}

	stats, err := db_ops.GetStats(db, name)
	if err != nil {
		fmt.Println(err)
		return db_ops.Link{}, errInternal
	}
	return stats.Link, nil
}

func createErrorStatus(err error) int {
	switch {
	case errors.Is(err, errInvalidURL), errors.Is(err, errInvalidName):
		return http.StatusBadRequest
	case errors.Is(err, db_ops.ErrNameTaken):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func listLinksHandler(w http.ResponseWriter, r *http.Request) {
	links, err := db_ops.ListURLs(db)
	if err != nil {
		fmt.Println(err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, links)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := db_ops.GetStats(db, r.PathValue("name"))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	if err != nil {
		fmt.Println(err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func deleteLinkHandler(w http.ResponseWriter, r *http.Request) {
	err := db_ops.DeleteURL(db, r.PathValue("name"))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	if err != nil {
		fmt.Println(err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
