package server

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/carlosmecha/todo/store"
)

// SizeLimit is the max size of the request body (1MB)
const SizeLimit = int64(1 * 1024 * 1024)

var (
	// ErrNoAuthProvided when the request doesn't have the auth token
	ErrNoAuthProvided = errors.New("no token auth provided")

	// ErrInvalidAuth when the token is invalid
	ErrInvalidAuth = errors.New("invalid token")
)

// handler takes care of the requests. Is a net/http.Handler
type handler struct {
	logger *log.Logger
	store  store.Store
	view   *template.Template
}

// RunServer starts the server listening in the specified address.
func RunServer(addr string, store store.Store, logger *log.Logger) *http.Server {

	h := &handler{
		store:  store,
		logger: logger,
		view:   template.Must(template.New("view").Parse(htmlView)),
	}

	server := &http.Server{
		Addr:    addr,
		Handler: h,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			h.logger.Fatalf("Server shutdown: %s", err.Error())
		}
	}()

	return server
}

// ServeHTTP is the main handler method.
func (h *handler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	h.logger.Printf("Request %s: %s, Content Length %d, Token %s", req.Method, req.URL.Path, req.ContentLength, req.Header.Get("Token"))
	defer req.Body.Close()

	if req.Method == "GET" && req.URL.Path == "/index.html" {
		h.getView(resp, req)
		h.logger.Printf("View served")
		return
	}

	switch req.Method {
	case "GET":
		h.get(resp, req)
	case "HEAD":
		h.head(resp, req)
	case "PUT":
		h.put(resp, req)
	default:
		h.logger.Printf("Invalid request, method not recognized")
		resp.WriteHeader(404)
	}

	h.logger.Printf("Request served")
}

// head retrieves the information about the file.
func (h *handler) head(resp http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "" && req.URL.Path != "/" {
		h.logger.Printf("Invalid path")
		resp.WriteHeader(404)
		return
	}

	version, err := h.store.GetCurrentVersion()
	if err != nil {
		h.logger.Printf("Error getting current version")
		resp.WriteHeader(500)
		return
	}

	resp.Header().Add("Last-Modified", version.Format(time.RFC1123))
	resp.WriteHeader(200)
}

// get returns the file.
func (h *handler) get(resp http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "":
		fallthrough
	case "/":
		var version time.Time
		date := req.Header.Get("If-Modified-Since")
		if date != "" {
			var err error
			version, err = time.Parse(time.RFC1123, date)
			if err != nil {
				h.logger.Printf("Unrecognized version date")
				resp.WriteHeader(400)
				return
			}
		}

		req.Header.Add("Content-Type", "text/plain; charset=utf-8")
		version, err := h.store.Get(version, resp)
		if err != nil {
			if err == store.ErrNotModified {
				h.logger.Printf("The requested version is the same")
				resp.WriteHeader(304)
				return
			} else if err == store.ErrVersionConflict {
				h.logger.Printf("The requested version is newer than the stored one")
				resp.WriteHeader(409)
				return
			}
			h.logger.Printf("Error getting file")
			resp.WriteHeader(500)
			return
		}
		resp.Header().Add("Last-Modified", version.Format(time.RFC1123))
	default:
		h.logger.Printf("Invalid path")
		resp.WriteHeader(404)
		return
	}

}

// getView returns the HTML content.
func (h *handler) getView(resp http.ResponseWriter, req *http.Request) {
	req.Header.Add("Content-Type", "text/html; charset=utf-8")
	buf := &bytes.Buffer{}
	if _, err := h.store.Get(time.Time{}, buf); err != nil {
		h.logger.Printf("Error getting file")
		resp.WriteHeader(500)
		return
	}

	if err := h.view.Execute(resp, struct{ Body string }{buf.String()}); err != nil {
		h.logger.Printf("Error getting view")
		resp.WriteHeader(500)
	}
}

func (h *handler) put(resp http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "" && req.URL.Path != "/" {
		h.logger.Printf("Invalid path")
		resp.WriteHeader(404)
		return
	}

	date := req.Header.Get("Last-Modified")
	version, err := time.Parse(time.RFC1123, date)
	if err != nil {
		h.logger.Printf("Unrecognized version date")
		resp.WriteHeader(400)
		return
	}

	if req.ContentLength <= 0 {
		h.logger.Printf("Missing body or content length")
		resp.WriteHeader(400)
		return
	}

	if req.ContentLength >= SizeLimit {
		h.logger.Printf("Body too large")
		resp.WriteHeader(413)
		return
	}

	reader, err := copyBody(req.Body)
	if err != nil {
		h.logger.Printf("Error reading body")
		resp.WriteHeader(500)
		return
	}

	force := req.Header.Get("Force")
	if force == "" || force == "false" {
		err = h.store.SafePut(version, req.ContentLength, reader)
	} else {
		h.logger.Printf("Requested FORCE put")
		err = h.store.Overwrite(req.ContentLength, reader)
	}

	if err != nil {
		if err != store.ErrVersionConflict {
			h.logger.Printf("Error writing file")
			resp.WriteHeader(500)
			return
		}
		h.logger.Printf("Version conflict writing file")
		resp.WriteHeader(409)
		return
	}

	resp.Header().Add("Last-Modified", version.Format(time.RFC1123))
	resp.WriteHeader(200)
}

func copyBody(body io.Reader) (*bytes.Reader, error) {
	content, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(content), nil
}
