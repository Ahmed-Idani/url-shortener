package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/Ahmed-Idani/bot"
	"github.com/Ahmed-Idani/clickinfo"
	"github.com/Ahmed-Idani/db_ops"
	"github.com/Ahmed-Idani/ip"
	"github.com/joho/godotenv"
	"github.com/phuslu/iploc"
)

var (
	db         *sql.DB
	adminToken string
	baseURL    string
	timezone   = time.UTC
)

func init(){
	if err:= db_ops.InitModels(); err != nil {
		panic(fmt.Errorf("failed to initialize models!: %w",err))
	}
}

func redirectHandler(w http.ResponseWriter, r *http.Request){
	urlID, target, err := db_ops.GetURL(db, r.PathValue("slug"))
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		fmt.Println(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	addr, err := ip.ClientIP(r)
	if err != nil {
		fmt.Println(err)
	} else {
		browser, osName, device := clickinfo.Parse(r.UserAgent())
		click := db_ops.Click{
			IP:        addr.String(),
			Origin:    bot.Origin(r.UserAgent()),
			Location:  iploc.IPCountry(addr),
			UserAgent: r.UserAgent(),
			Browser:   browser,
			OS:        osName,
			Device:    device,
			Referrer:  clickinfo.ReferrerHost(r.Referer()),
		}
		if err := db_ops.InsertClick(db, urlID, click); err != nil {
			fmt.Println(err)
		}
	}

	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func main(){
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	if err := godotenv.Load(".env." + env); err != nil {
		fmt.Printf("error loading .env.%s! %s\n", env, err)
		os.Exit(1)
	}
	for _, key := range []string{"ADDR", "ADMIN_TOKEN", "BASE_URL"} {
		if os.Getenv(key) == "" {
			fmt.Printf("%s is not set in .env.%s\n", key, env)
			os.Exit(1)
		}
	}
	addr := os.Getenv("ADDR")
	adminToken = os.Getenv("ADMIN_TOKEN")
	baseURL = strings.TrimRight(os.Getenv("BASE_URL"), "/")
	if tz := os.Getenv("TIMEZONE"); tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			fmt.Printf("invalid TIMEZONE %q in .env.%s! %s\n", tz, env, err)
			os.Exit(1)
		}
		timezone = loc
	}

	var err error
	db, err = db_ops.OpenBD()
	if err != nil {
		fmt.Printf("error opening database! %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	http.HandleFunc("GET /{slug}", redirectHandler)

	http.HandleFunc("POST /api/public/login", loginSubmit)
	http.HandleFunc("POST /api/public/logout", logoutSubmit)

	private := http.NewServeMux()
	private.HandleFunc("POST /links", createLinkHandler)
	private.HandleFunc("GET /links", listLinksHandler)
	private.HandleFunc("GET /links/{name}/stats", statsHandler)
	private.HandleFunc("DELETE /links/{name}", deleteLinkHandler)
	private.HandleFunc("POST /links/{name}/delete", deleteLinkSubmit)
	http.Handle("/api/private/", http.StripPrefix("/api/private", requireAuth(private)))

	registerDashboard()
	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	err = srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed){
		fmt.Printf("server closer \n")
	}else if err != nil {
		fmt.Printf("error starting server! %s\n", err)
		os.Exit(1)
	}
}
