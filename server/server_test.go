package server

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/carlosmecha/todo/store"
	"github.com/carlosmecha/todo/util/testutil"
)

type mockStore struct {
	version time.Time
	file    []byte
	view    []byte
	t       *testing.T
}

// GetCurrentVersion retrieves the version stored.
func (m *mockStore) GetCurrentVersion() (time.Time, error) {
	return m.version, nil
}

func (m *mockStore) Get(version time.Time, writer io.Writer) (time.Time, error) {
	m.t.Logf("Requested get %v", version.Format(time.RFC1123))
	if version.Before(m.version) {
		_, err := writer.Write(m.file)
		return m.version, err
	} else if version.Equal(m.version) {
		return m.version, store.ErrNotModified
	}
	return m.version, store.ErrVersionConflict
}

func (m *mockStore) GetHTMLView(writer io.Writer) error {
	_, err := writer.Write(m.view)
	return err
}

func (m *mockStore) SafePut(version time.Time, reader io.Reader) error {
	if version.After(m.version) {
		var err error
		m.file, err = ioutil.ReadAll(reader)
		m.version = version
		return err
	}
	return store.ErrVersionConflict
}

func (m *mockStore) Overwrite(reader io.Reader) error {
	var err error
	m.file, err = ioutil.ReadAll(reader)
	m.version = time.Now()
	return err
}

func TestRun(t *testing.T) {

	server := RunServer("token", ":0", &mockStore{})

	time.Sleep(1)
	if err := server.Shutdown(nil); err != nil {
		t.Fatal(err)
	}

}

func TestGet(t *testing.T) {

	currentVersion := time.Now().Format(time.RFC1123)
	version, _ := time.Parse(time.RFC1123, currentVersion)

	mock := &mockStore{
		version: version,
		file:    []byte("Hola"),
		view:    []byte("<html></htmll"),
		t:       t,
	}

	cases := []struct {
		token        string
		path         string
		version      string
		expectedCode int
	}{
		// OK
		{
			token:        "test",
			path:         "/",
			expectedCode: 200,
		},
		// OK
		{
			token:        "test",
			path:         "/view.html",
			expectedCode: 200,
		},
		// Missing Auth
		{
			token:        "",
			path:         "/",
			expectedCode: 401,
		},
		// Invalid Auth
		{
			token:        "token",
			path:         "/",
			expectedCode: 401,
		},
		// Invalid Path
		{
			token:        "test",
			path:         "/foo",
			expectedCode: 404,
		},
		// Not modified
		{
			token:        "test",
			path:         "/",
			expectedCode: 304,
			version:      mock.version.Format(time.RFC1123),
		},
		// Newer date
		{
			token:        "test",
			path:         "/",
			expectedCode: 409,
			version:      mock.version.AddDate(1, 0, 0).Format(time.RFC1123),
		},
		// Invalid date
		{
			token:        "test",
			path:         "/",
			expectedCode: 400,
			version:      "foo",
		},
	}

	server, addr := testServer("test", mock, t)
	defer func() {
		if err := server.Shutdown(nil); err != nil {
			t.Fatal(err)
		}
	}()

	client := &http.Client{}

	for _, c := range cases {
		req, err := http.NewRequest("GET", addr+c.path, nil)
		if err != nil {
			t.Fatal(err)
		}

		if c.token != "" {
			req.Header.Add("Token", c.token)
		}

		if c.version != "" {
			req.Header.Add("If-Modified-Since", c.version)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		resp.Body.Close()
		if resp.StatusCode != c.expectedCode {
			t.Fatalf("Expected %d status, got %d for case %+v", c.expectedCode, resp.StatusCode, c)
		}
	}

}

func TestHead(t *testing.T) {

	currentVersion := time.Now().Format(time.RFC1123)
	version, _ := time.Parse(time.RFC1123, currentVersion)

	mock := &mockStore{
		version: version,
		file:    []byte("Hola"),
		view:    []byte("<html></htmll"),
		t:       t,
	}

	cases := []struct {
		token           string
		path            string
		expectedVersion string
		expectedCode    int
	}{
		// OK
		{
			token:           "test",
			path:            "/",
			expectedCode:    200,
			expectedVersion: mock.version.Format(time.RFC1123),
		},
		// Missing Auth
		{
			token:        "",
			path:         "/",
			expectedCode: 401,
		},
		// Invalid Auth
		{
			token:        "token",
			path:         "/",
			expectedCode: 401,
		},
		// Invalid Path
		{
			token:        "test",
			path:         "/foo",
			expectedCode: 404,
		},
	}

	server, addr := testServer("test", mock, t)
	defer func() {
		if err := server.Shutdown(nil); err != nil {
			t.Fatal(err)
		}
	}()

	client := &http.Client{}

	for _, c := range cases {
		req, err := http.NewRequest("HEAD", addr+c.path, nil)
		if err != nil {
			t.Fatal(err)
		}

		if c.token != "" {
			req.Header.Add("Token", c.token)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		resp.Body.Close()
		if resp.StatusCode != c.expectedCode {
			t.Fatalf("Expected %d status, got %d for case %+v", c.expectedCode, resp.StatusCode, c)
		}

		if c.expectedVersion != "" && resp.Header.Get("Last-Modified") != c.expectedVersion {
			t.Fatalf("Expected version %s, got %s for case %+v", c.expectedVersion, resp.Header.Get("Last-Modified"), c)
		}
	}

}

func TestPut(t *testing.T) {

	now := time.Now()
	currentVersion := now.Format(time.RFC1123)
	version, _ := time.Parse(time.RFC1123, currentVersion)

	mock := &mockStore{t: t}

	cases := []struct {
		storedBody      []byte
		storedVersion   time.Time
		token           string
		path            string
		body            []byte
		version         string
		force           bool
		expectedCode    int
		expectedVersion string
		expectedBody    []byte
	}{
		// OK
		{
			storedBody:      []byte("hola"),
			storedVersion:   version,
			token:           "test",
			path:            "/",
			body:            []byte("adios"),
			version:         now.AddDate(0, 0, 1).Format(time.RFC1123),
			expectedCode:    200,
			expectedVersion: now.AddDate(0, 0, 1).Format(time.RFC1123),
			expectedBody:    []byte("adios"),
		},
		// Version conflict
		{
			storedBody:      []byte("hola"),
			storedVersion:   version,
			token:           "test",
			path:            "/",
			body:            []byte("adios"),
			version:         now.AddDate(-1, 0, 0).Format(time.RFC1123),
			expectedCode:    409,
			expectedVersion: currentVersion,
			expectedBody:    []byte("hola"),
		},
		// Force
		{
			storedBody:    []byte("hola"),
			storedVersion: version,
			token:         "test",
			path:          "/",
			body:          []byte("adios"),
			version:       now.AddDate(0, 0, -1).Format(time.RFC1123),
			force:         true,
			expectedCode:  200,
			expectedBody:  []byte("adios"),
		},
		// Missing Auth
		{
			token:        "",
			path:         "/",
			expectedCode: 401,
		},
		// Invalid Auth
		{
			token:        "token",
			path:         "/",
			expectedCode: 401,
		},
		// Invalid Path
		{
			token:        "test",
			path:         "/foo",
			expectedCode: 404,
		},
		// Invalid date
		{
			token:        "test",
			path:         "/",
			expectedCode: 400,
			version:      "foo",
		},
		// Invalid body
		{
			storedBody:    []byte("hola"),
			storedVersion: version,
			token:         "test",
			path:          "/",
			version:       now.AddDate(0, 0, -1).Format(time.RFC1123),
			expectedCode:  400,
		},
	}

	server, addr := testServer("test", mock, t)
	defer func() {
		if err := server.Shutdown(nil); err != nil {
			t.Fatal(err)
		}
	}()

	client := &http.Client{}

	for _, c := range cases {
		mock.file = c.storedBody
		mock.version = c.storedVersion

		req, err := http.NewRequest("PUT", addr+c.path, nil)
		if err != nil {
			t.Fatal(err)
		}

		if c.token != "" {
			req.Header.Add("Token", c.token)
		}

		if c.version != "" {
			req.Header.Add("Last-Modified", c.version)
		}

		if c.force {
			req.Header.Add("Force", "true")
		}

		if len(c.body) > 0 {
			req.Body = testutil.NewBufferCloser(c.body)
			req.ContentLength = int64(len(c.body))
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		resp.Body.Close()
		if resp.StatusCode != c.expectedCode {
			t.Fatalf("Expected %d status, got %d for case %+v", c.expectedCode, resp.StatusCode, c)
		}

		if c.expectedVersion != "" && mock.version.Format(time.RFC1123) != c.expectedVersion {
			t.Fatalf("Expected version %s, got %s for case %+v", c.expectedVersion, mock.version.Format(time.RFC1123), c)
		}

		if len(c.expectedBody) > 0 {
			if err != nil {
				t.Fatal(err)
			}

			if string(mock.file) != string(c.expectedBody) {
				t.Fatalf("Expected body %s, got %s for case %+v", string(c.expectedBody), string(mock.file), c)
			}
		}

	}

}

func testServer(token string, store store.Store, t *testing.T) (*http.Server, string) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	h := &handler{
		authToken: token,
		store:     store,
		logger:    log.New(os.Stdout, "", log.LstdFlags),
	}

	server := &http.Server{
		Handler: h,
	}

	go func() {
		if err := server.Serve(listener); err != nil {
			t.Fatal(err)
		}
	}()

	return server, fmt.Sprintf("http://localhost:%d", port)
}
