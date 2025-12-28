package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ReanSn0w/gokit/pkg/app"
	"github.com/ReanSn0w/pikpak-webdav-proxy/pkg/fs"
	"github.com/studio-b12/gowebdav"
	"golang.org/x/net/webdav"
)

var (
	revision = "debug"

	opts = struct {
		app.Debug

		Port      string `long:"port" env:"PORT" default:"8080" description:"Порт для WebDAV сервера"`
		LocalPath string `long:"local-path" env:"LOCAL_PATH" default:"/cache" description:"Путь к директории кеша"`

		Webdav struct {
			URL  string `long:"url" env:"URL" default:"https://dav.mypikpak.com" description:"URL для WebDAV сервера"`
			User string `long:"user" env:"USER" description:"Пользователь для WebDAV сервера"`
			Pass string `long:"pass" env:"PASS" description:"Пароль для WebDAV сервера"`
		} `group:"Target Server" namespace:"webdav" env-namespace:"WEBDAV"`
	}{}
)

func main() {
	app := app.New("Webdav Proxy", revision, &opts)

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
	handler := &webdav.Handler{
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				log.Printf("[%s] %s -> ERR: %v", r.Method, r.URL.Path, err)
			} else {
				log.Printf("[%s] %s -> OK", r.Method, r.URL.Path)
			}
		},
	}

	http.Handle("/", handler)

	addr := fmt.Sprintf(":%s", opts.Port)
	log.Printf("WebDAV сервер запущен на %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
