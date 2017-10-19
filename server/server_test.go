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
	if !version.Equal(m.version) {
		_, err := writer.Write(m.file)
		return m.version, err
	}
	return m.version, store.ErrNotModified
}

func (m *mockStore) GetHTMLView(writer io.Writer) error {
	_, err := writer.Write(m.view)
	return err
}

func (m *mockStore) SafePut(version time.Time, reader io.Reader) error {
	if version.After(m.version) {
		return m.Overwrite(reader)
	}
	return store.ErrVersionConflict
}

func (m *mockStore) Overwrite(reader io.Reader) error {
	var err error
	m.file, err = ioutil.ReadAll(reader)
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
