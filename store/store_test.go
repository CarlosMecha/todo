package store

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/carlosmecha/todo/util/testutil"
)

type s3mock struct {
	data    map[string][]byte
	version map[string]string
	t       *testing.T

	s3iface.S3API
}

func (m *s3mock) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	url := fmt.Sprintf("s3://%s/%s", *input.Bucket, *input.Key)
	m.t.Logf("Called GetObject %s", url)
	if _, ok := m.data[url]; !ok {
		return nil, awserr.New(s3.ErrCodeNoSuchKey, "not found", ErrNotFound)
	}

	buffer := testutil.NewBufferCloser(m.data[url])

	return &s3.GetObjectOutput{
		Body:          buffer,
		ContentLength: aws.Int64(int64(len(m.data[url]))),
		ContentType:   aws.String("text/plan"),
		Metadata:      map[string]*string{"version": aws.String(m.version[url])},
	}, nil
}

func (m *s3mock) HeadObject(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
	url := fmt.Sprintf("s3://%s/%s", *input.Bucket, *input.Key)
	m.t.Logf("Called HeadObject %s", url)
	if _, ok := m.data[url]; !ok {
		return nil, awserr.New(s3.ErrCodeNoSuchKey, "not found", ErrNotFound)
	}

	return &s3.HeadObjectOutput{
		ContentLength: aws.Int64(int64(len(m.data[url]))),
		ContentType:   aws.String("text/plan"),
		Metadata:      map[string]*string{"version": aws.String(m.version[url])},
	}, nil
}

func (m *s3mock) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	url := fmt.Sprintf("s3://%s/%s", *input.Bucket, *input.Key)
	m.t.Logf("Called PutObject %s", url)
	b := new(bytes.Buffer)
	if _, err := b.ReadFrom(input.Body); err != nil {
		return nil, err
	}

	if m.data == nil {
		m.data = make(map[string][]byte)
	}

	m.data[url] = b.Bytes()
	m.version[url] = *input.Metadata["version"]

	return &s3.PutObjectOutput{}, nil
}

func TestGetCurrentVersion(t *testing.T) {

	currentVersion := time.Now().Format(time.RFC1123)
	version, _ := time.Parse(time.RFC1123, currentVersion)

	cases := []struct {
		key             string
		bucket          string
		expectedVersion time.Time
		expectedError   error
	}{
		// OK
		{
			key:             "test",
			bucket:          "test",
			expectedVersion: version,
		},
		// Not found
		{
			key:           "foo",
			bucket:        "bar",
			expectedError: ErrNotFound,
		},
		// Missing version
		{
			key:           "test2",
			bucket:        "test",
			expectedError: ErrInvalidVersion,
		},
	}

	mock := &s3mock{
		data: map[string][]byte{
			"s3://test/test":  []byte(""),
			"s3://test/test2": []byte(""),
		},
		version: map[string]string{
			"s3://test/test": version.Format(time.RFC1123),
		},
		t: t,
	}

	for _, c := range cases {
		s := &store{
			key:    aws.String(c.key),
			bucket: aws.String(c.bucket),
			logger: log.New(os.Stdout, "", log.LstdFlags),
			s3:     mock,
		}

		if got, err := s.GetCurrentVersion(); err != nil {
			if c.expectedError == nil {
				t.Fatalf("Unexpected error %s", err.Error())
			} else if c.expectedError != err {
				t.Fatalf("Expected error %s, got %s", c.expectedError.Error(), err.Error())
			}
		} else if !got.Equal(c.expectedVersion) {
			t.Fatalf("Expected version %s, got %s", c.expectedVersion.Format(time.RFC1123), got.Format(time.RFC1123))
		}

	}

}

func TestGet(t *testing.T) {

	now := time.Now()
	currentVersion := now.Format(time.RFC1123)
	version, _ := time.Parse(time.RFC1123, currentVersion)

	cases := []struct {
		key             string
		bucket          string
		version         time.Time
		expectedBody    []byte
		expectedVersion time.Time
		expectedError   error
	}{
		// OK
		{
			key:             "test",
			bucket:          "test",
			version:         now.AddDate(-1, 0, 0),
			expectedBody:    []byte("hola"),
			expectedVersion: version,
		},
		// Not found
		{
			key:           "foo",
			bucket:        "bar",
			version:       version,
			expectedError: ErrNotFound,
		},
		// Missing version
		{
			key:           "test2",
			bucket:        "test",
			version:       version,
			expectedError: ErrInvalidVersion,
		},
		// Same version
		{
			key:           "test",
			bucket:        "test",
			version:       version,
			expectedError: ErrNotModified,
		},
		// Newer version
		{
			key:           "test",
			bucket:        "test",
			version:       version.AddDate(1, 0, 0),
			expectedError: ErrVersionConflict,
		},
	}

	mock := &s3mock{
		data: map[string][]byte{
			"s3://test/test":  []byte("hola"),
			"s3://test/test2": []byte(""),
		},
		version: map[string]string{
			"s3://test/test": version.Format(time.RFC1123),
		},
		t: t,
	}

	for _, c := range cases {
		s := &store{
			key:    aws.String(c.key),
			bucket: aws.String(c.bucket),
			logger: log.New(os.Stdout, "", log.LstdFlags),
			s3:     mock,
		}

		buff := &bytes.Buffer{}

		if got, err := s.Get(c.version, buff); err != nil {
			if c.expectedError == nil {
				t.Fatalf("Unexpected error %s", err.Error())
			} else if c.expectedError != err {
				t.Fatalf("Expected error %s, got %s", c.expectedError.Error(), err.Error())
			}
		} else if !got.Equal(c.expectedVersion) {
			t.Fatalf("Expected version %s, got %s", c.expectedVersion.Format(time.RFC1123), got.Format(time.RFC1123))
		} else if string(c.expectedBody) != buff.String() {
			t.Fatalf("Expected %s, got %s", string(c.expectedBody), buff.String())
		}

	}

}

func TestGetHTMLView(t *testing.T) {

	now := time.Now()
	currentVersion := now.Format(time.RFC1123)
	version, _ := time.Parse(time.RFC1123, currentVersion)

	cases := []struct {
		key           string
		bucket        string
		expectedBody  []byte
		expectedError error
	}{
		// OK
		{
			key:          "test",
			bucket:       "test",
			expectedBody: []byte("hola"),
		},
		// Not found
		{
			key:           "foo",
			bucket:        "bar",
			expectedError: ErrNotFound,
		},
	}

	mock := &s3mock{
		data: map[string][]byte{
			"s3://test/test": []byte("hola"),
		},
		version: map[string]string{
			"s3://test/test": version.Format(time.RFC1123),
		},
		t: t,
	}

	for _, c := range cases {
		s := &store{
			key:    aws.String(c.key),
			bucket: aws.String(c.bucket),
			logger: log.New(os.Stdout, "", log.LstdFlags),
			s3:     mock,
		}

		buff := &bytes.Buffer{}

		if err := s.GetHTMLView(buff); err != nil {
			if c.expectedError == nil {
				t.Fatalf("Unexpected error %s", err.Error())
			} else if c.expectedError != err {
				t.Fatalf("Expected error %s, got %s", c.expectedError.Error(), err.Error())
			}
			continue
		}

		got := buff.String()
		expected := fmt.Sprintf(htmlView, base64.StdEncoding.EncodeToString(c.expectedBody))
		if expected != got {
			t.Fatalf("Expected %s, got %s", expected, got)
		}

	}

}

func TestSafePut(t *testing.T) {

	now := time.Now()
	currentVersion := now.Format(time.RFC1123)
	version, _ := time.Parse(time.RFC1123, currentVersion)

	cases := []struct {
		key             string
		bucket          string
		version         time.Time
		body            []byte
		expectedVersion time.Time
		expectedBody    []byte
		expectedError   error
	}{
		// OK
		{
			key:             "test",
			bucket:          "test",
			version:         version.AddDate(0, 0, 1),
			body:            []byte("adios"),
			expectedVersion: version.AddDate(0, 0, 1),
			expectedBody:    []byte("adios"),
		},
		// Not found
		{
			key:             "foo",
			bucket:          "bar",
			version:         version.AddDate(0, 0, 1),
			body:            []byte("adios"),
			expectedVersion: version.AddDate(0, 0, 1),
			expectedBody:    []byte("adios"),
		},
		// Same date
		{
			key:           "test",
			bucket:        "test",
			version:       version,
			body:          []byte("adios"),
			expectedError: ErrVersionConflict,
		},
		// Older date
		{
			key:           "test",
			bucket:        "test",
			version:       version.AddDate(0, 0, -1),
			body:          []byte("adios"),
			expectedError: ErrVersionConflict,
		},
	}

	mock := &s3mock{
		data: map[string][]byte{
			"s3://test/test": []byte("hola"),
		},
		version: map[string]string{
			"s3://test/test": version.Format(time.RFC1123),
		},
		t: t,
	}

	for _, c := range cases {
		s := &store{
			key:    aws.String(c.key),
			bucket: aws.String(c.bucket),
			logger: log.New(os.Stdout, "", log.LstdFlags),
			s3:     mock,
		}

		buff := bytes.NewBuffer(c.body)

		if err := s.SafePut(c.version, buff); err != nil {
			if c.expectedError == nil {
				t.Fatalf("Unexpected error %s", err.Error())
			} else if c.expectedError != err {
				t.Fatalf("Expected error %s, got %s", c.expectedError.Error(), err.Error())
			}
			continue
		}

		url := fmt.Sprintf("s3://%s/%s", c.bucket, c.key)
		gotVersion := mock.version[url]
		gotBody := mock.data[url]

		if gotVersion != c.expectedVersion.Format(time.RFC1123) {
			t.Fatalf("Expected version %s, got %s", c.expectedVersion.Format(time.RFC1123), gotVersion)
		}

		if string(c.expectedBody) != string(gotBody) {
			t.Fatalf("Expected %s, got %s", string(c.expectedBody), string(gotBody))
		}

	}
}

func TestOverwrite(t *testing.T) {
	now := time.Now()
	currentVersion := now.Format(time.RFC1123)
	version, _ := time.Parse(time.RFC1123, currentVersion)

	cases := []struct {
		key          string
		bucket       string
		body         []byte
		expectedBody []byte
	}{
		// OK
		{
			key:          "test",
			bucket:       "test",
			body:         []byte("adios"),
			expectedBody: []byte("adios"),
		},
		// Not found
		{
			key:          "foo",
			bucket:       "bar",
			body:         []byte("adios"),
			expectedBody: []byte("adios"),
		},
	}

	mock := &s3mock{
		data: map[string][]byte{
			"s3://test/test": []byte("hola"),
		},
		version: map[string]string{
			"s3://test/test": version.Format(time.RFC1123),
		},
		t: t,
	}

	for _, c := range cases {
		s := &store{
			key:    aws.String(c.key),
			bucket: aws.String(c.bucket),
			logger: log.New(os.Stdout, "", log.LstdFlags),
			s3:     mock,
		}

		buff := bytes.NewBuffer(c.body)

		if err := s.Overwrite(buff); err != nil {
			t.Fatalf("Unexpected error %s", err.Error())
		}

		url := fmt.Sprintf("s3://%s/%s", c.bucket, c.key)
		gotBody := mock.data[url]

		if string(c.expectedBody) != string(gotBody) {
			t.Fatalf("Expected %s, got %s", string(c.expectedBody), string(gotBody))
		}

	}
}
