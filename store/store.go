package store

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

// Version is the metadata field name
const Version = "Version"

var (
	// ErrNotModified is returned when the stored version is the same as provided
	ErrNotModified = errors.New("not modified")

	// ErrVersionConflict when it's trying to update a file with an old version
	ErrVersionConflict = errors.New("version conflict")

	// ErrInvalidVersion when the stored version is missing or invalid
	ErrInvalidVersion = errors.New("invalid version")

	// ErrNotFound when the file is not found
	ErrNotFound = errors.New("not found")

	contentType = aws.String("text/plain")
)

// Store retrieves and updates the TODO list
type Store interface {

	// GetCurrentVersion retrieves the version stored.
	GetCurrentVersion() (time.Time, error)

	// Get retrieves the file
	Get(time.Time, io.Writer) (time.Time, error)

	// SafePut overwrites the file if the new version is newer than the stored one.
	SafePut(time.Time, int64, io.ReadSeeker) error

	// Overwrite overwrites the version stored.
	Overwrite(int64, io.ReadSeeker) error
}

// store uses S3 to store the files
type store struct {
	s3     s3iface.S3API
	bucket *string
	key    *string
	logger *log.Logger
}

// NewStore creates a new store using the provided key and bucket
func NewStore(bucket, key, region string, logger *log.Logger) *store {
	s3Client := s3.New(session.New(&aws.Config{
		Region:     aws.String(region),
		MaxRetries: aws.Int(5),
	}))

	return &store{
		s3:     s3Client,
		bucket: aws.String(bucket),
		key:    aws.String(key),
		logger: logger,
	}
}

// GetCurrentVersion retrieves the version stored.
func (s *store) GetCurrentVersion() (time.Time, error) {
	resp, err := s.s3.HeadObject(&s3.HeadObjectInput{
		Bucket: s.bucket,
		Key:    s.key,
	})

	if err != nil {
		if isNotFound(err) {
			s.logger.Print("File not found")
			return time.Time{}, ErrNotFound
		}
		s.logger.Printf("Error getting file info: %s", err.Error())
		return time.Time{}, err
	}

	metadata, found := resp.Metadata[Version]
	if !found {
		s.logger.Printf("Missing stored version, found metadata %+v", resp.Metadata)
		return time.Time{}, ErrInvalidVersion
	}

	version, err := time.Parse(time.RFC1123, *metadata)
	if err != nil {
		s.logger.Printf("Invalid stored version: %s", err.Error())
		return time.Time{}, ErrInvalidVersion
	}

	return version, nil
}

// Get retrieves the file
func (s *store) Get(version time.Time, writer io.Writer) (time.Time, error) {
	resp, err := s.s3.GetObject(&s3.GetObjectInput{
		Bucket: s.bucket,
		Key:    s.key,
	})
	if err != nil {
		if isNotFound(err) {
			s.logger.Print("File not found")
			return time.Time{}, ErrNotFound
		}
		s.logger.Printf("Error getting file: %s", err.Error())
		return time.Time{}, err
	}
	defer resp.Body.Close()

	metadata, found := resp.Metadata[Version]
	if !found {
		s.logger.Print("Missing stored version")
		return time.Time{}, ErrInvalidVersion
	}

	currentVersion, err := time.Parse(time.RFC1123, *metadata)
	if err != nil {
		s.logger.Printf("Invalid stored version: %s", err.Error())
		return time.Time{}, ErrInvalidVersion
	}

	if currentVersion.After(version) {
		// Read all in memory
		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			s.logger.Printf("Error reading file: %s", err.Error())
			return time.Time{}, err
		}

		if _, err := writer.Write(content); err != nil {
			s.logger.Printf("Error writing file: %s", err.Error())
			return time.Time{}, err
		}
	} else if currentVersion.Equal(version) {
		s.logger.Print("The provided version is same as the content")
		return time.Time{}, ErrNotModified
	} else {
		s.logger.Print("The provided version is newer than the content")
		return time.Time{}, ErrVersionConflict
	}

	return currentVersion, nil
}

// SafePut overwrites the file if the new version is newer than the stored one.
func (s *store) SafePut(version time.Time, contentLength int64, reader io.ReadSeeker) error {
	currentVersion, err := s.GetCurrentVersion()
	if err != nil {
		if err != ErrNotFound {
			return err
		}
		currentVersion = time.Time{}
	}

	if currentVersion.Before(version) {
		return s.write(version, contentLength, reader)
	}

	s.logger.Printf("Version conflict, the stored version is newer")
	return ErrVersionConflict
}

// Overwrite overwrites the version stored.
func (s *store) Overwrite(contentLength int64, reader io.ReadSeeker) error {
	return s.write(time.Now(), contentLength, reader)
}

func (s *store) write(version time.Time, contentLength int64, reader io.ReadSeeker) error {
	if _, err := s.s3.PutObject(&s3.PutObjectInput{
		Body:          reader,
		Bucket:        s.bucket,
		Key:           s.key,
		ContentType:   contentType,
		ContentLength: aws.Int64(contentLength),
		Metadata:      map[string]*string{Version: aws.String(version.Format(time.RFC1123))},
	}); err != nil {
		s.logger.Printf("Can't store the file: %s", err.Error())
		return err
	}

	return nil
}

func isNotFound(err error) bool {
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == s3.ErrCodeNoSuchKey {
		return true
	}
	if aerr, ok := err.(awserr.RequestFailure); ok && aerr.StatusCode() == 404 {
		return true
	}
	return false
}
