package server

import (
	"bytes"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/kevinburke/handlers"
	"github.com/kevinburke/rest"
	"github.com/saintpete/logrole/assets"
)

var messageTemplate *template.Template

func init() {
	idx := assets.MustAsset("templates/sms.html")
	messageTemplate = template.Must(template.New("messages").Parse(string(idx))).Option("missingkey=error")
	staticServer = &static{
		modTime: time.Now().UTC(),
	}
}

type static struct {
	modTime time.Time
}

type server struct {
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := messageTemplate.Execute(w, nil); err != nil {
		rest.ServerError(w, r, err)
	}
}

func UpgradeInsecureHandler(h http.Handler, allowUnencryptedTraffic bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowUnencryptedTraffic == false {
			if r.Header.Get("X-Forwarded-Proto") == "http" {
				u := r.URL
				u.Scheme = "https"
				u.Host = r.Host
				http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
				return
			}
		}
		// This header doesn't mean anything when served over HTTP, but
		// detecting HTTPS is a general way is hard, so let's just send it
		// every time.
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		h.ServeHTTP(w, r)
	})
}

var staticServer http.Handler

func (s *static) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bits, err := assets.Asset(strings.TrimPrefix(r.URL.Path, "/"))
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		rest.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, r.URL.Path, s.modTime, bytes.NewReader(bits))
}

// NewServer returns a new Handler that can serve requests. If the users map is
// empty, Basic Authentication is disabled.
func NewServer(allowUnencryptedTraffic bool, users map[string]string) http.Handler {
	s := &server{}
	r := new(handlers.Regexp)
	r.Handle(regexp.MustCompile(`^/messages$`), []string{"GET"}, s)
	r.Handle(regexp.MustCompile(`^/static`), []string{"GET"}, staticServer)
	var h http.Handler = r
	if len(users) > 0 {
		h = handlers.BasicAuth(r, "logrole", users)
	}
	return UpgradeInsecureHandler(h, allowUnencryptedTraffic)
}