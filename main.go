package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ReanSn0w/gokit/pkg/app"
	"github.com/ReanSn0w/pikpak-webdav-proxy/pkg/fs"
	"github.com/go-pkgz/lgr"
	"github.com/studio-b12/gowebdav"
	"golang.org/x/net/webdav"
)

var (
	revision = "debug"

	opts = struct {
		app.Debug

		Port      string `long:"port" env:"PORT" default:"8080" description:"Порт для WebDAV сервера"`
		LocalPath string `long:"local-path" env:"LOCAL_PATH" default:"/cache" description:"Путь к директории кеша"`

		Auth struct {
			Enabled bool   `long:"enabled" env:"ENABLED" description:"Включить аутентификацию"`
			User    string `long:"user" env:"USER" description:"Пользователь для аутентификации"`
			Pass    string `long:"pass" env:"PASS" description:"Пароль для аутентификации"`
		} `group:"Authentication" namespace:"auth" env-namespace:"AUTH"`

		Webdav struct {
			URL  string `long:"url" env:"URL" default:"https://dav.mypikpak.com" description:"URL для WebDAV сервера"`
			User string `long:"user" env:"USER" description:"Пользователь для WebDAV сервера"`
			Pass string `long:"pass" env:"PASS" description:"Пароль для WebDAV сервера"`
		} `group:"Target Server" namespace:"webdav" env-namespace:"WEBDAV"`
	}{}
)

func main() {
	app := app.New("Webdav Proxy", revision, &opts)

	if opts.Auth.Enabled {
		if opts.Auth.User == "" || opts.Auth.Pass == "" {
			app.Log().Logf("[ERROR] auth user or pass is empty")
			os.Exit(1)
		}
	}

	wd := gowebdav.NewClient(opts.Webdav.URL, opts.Webdav.User, opts.Webdav.Pass)
	err := wd.Connect()
	if err != nil {
		app.Log().Logf("[ERROR] webdav error: %v", err)
		os.Exit(2)
	}

	// Создаём директорию кеша
	os.MkdirAll(opts.LocalPath, 0755)

	// Создаём proxy filesystem
	fs := fs.NewPikpakProxy(app.Log(), opts.LocalPath, wd)

	// WebDAV обработчик
	handler := webdavHandler(app.Log(), fs)

	// Middleware для авторизации
	http.Handle("/", authMiddleware(handler))

	addr := fmt.Sprintf(":%s", opts.Port)
	log.Printf("WebDAV сервер запущен на %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if opts.Auth.Enabled {
			user, pass, ok := r.BasicAuth()
			if !ok || user != opts.Auth.User || pass != opts.Auth.Pass {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func webdavHandler(log lgr.L, fs webdav.FileSystem) http.Handler {
	return &webdav.Handler{
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				log.Logf("[ERROR] [%s] %s -> ERR: %v", r.Method, r.URL.Path, err)
			} else {
				log.Logf("[INFO] [%s] %s -> OK", r.Method, r.URL.Path)
			}
		},
	}
}
