package store

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

const htmlView = `
<!DOCTYPE html>
<html>
    <head>
        <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
        <script type="text/javascript" src="https://cdnjs.cloudflare.com/ajax/libs/showdown/1.7.6/showdown.min.js"></script>
        <title>TODO</title>
    </head>
    <body>
        <div id="file" hidden>%s</div>
        <div id="view" style="width: 600px; padding: 0 10px"></div>
        <script type="text/javascript">
        (function (d, s){
            var file = d.getElementById("file").textContent;
            d.getElementById("view").innerHTML = (new s.Converter()).makeHtml(atob(file));
        })(document, showdown)
      </script>
    </body>
</html>
`

var (
	// ErrNotModified is returned when the stored version is the same as provided
	ErrNotModified = errors.New("not modified")

	// ErrVersionConflict when it's trying to update a file with an old version
	ErrVersionConflict = errors.New("version conflict")

	// ErrInvalidVersion when the stored version is missing or invalid
	ErrInvalidVersion = errors.New("invalid version")

	// ErrNotFound when the file is not found
	ErrNotFound = errors.New("not found")

	contentEncoding = aws.String("")

	contentType = aws.String("")
)

// Store retrieves and updates the TODO list
type Store interface {

	// GetCurrentVersion retrieves the version stored.
	GetCurrentVersion() (time.Time, error)

	// Get retrieves the file
	Get(time.Time, io.Writer) (time.Time, error)

	// GetHTMLView returns the HTML view of the file.
	GetHTMLView(io.Writer) error

	// SafePut overwrites the file if the new version is newer than the stored one.
	SafePut(time.Time, io.Reader) error

	// Overwrite overwrites the version stored.
	Overwrite(io.Reader) error
}

// store uses S3 to store the files
type store struct {
	s3     s3iface.S3API
	bucket *string
	key    *string
	logger *log.Logger
}

// GetCurrentVersion retrieves the version stored.
func (s *store) GetCurrentVersion() (time.Time, error) {
	resp, err := s.s3.HeadObject(&s3.HeadObjectInput{
		Bucket: s.bucket,
		Key:    s.key,
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == s3.ErrCodeNoSuchKey {
			s.logger.Print("File not found")
			return time.Time{}, ErrNotFound
		}
		s.logger.Printf("Error getting file info: %s", err.Error())
		return time.Time{}, err
	}

	metadata, found := resp.Metadata["version"]
	if !found {
		s.logger.Print("Missing stored version")
		return time.Time{}, ErrInvalidVersion
	}

	version, err := time.Parse(*metadata, time.RFC1123)
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
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == s3.ErrCodeNoSuchKey {
			s.logger.Print("File not found")
			return time.Time{}, ErrNotFound
		}
		s.logger.Printf("Error getting file: %s", err.Error())
		return time.Time{}, err
	}
	defer resp.Body.Close()

	metadata, found := resp.Metadata["version"]
	if !found {
		s.logger.Print("Missing stored version")
		return time.Time{}, ErrInvalidVersion
	}

	currentVersion, err := time.Parse(*metadata, time.RFC1123)
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
	} else {
		s.logger.Print("The provided version is newer than the content")
	}

	return currentVersion, nil
}

// GetHTMLView returns the HTML view of the file.
func (s *store) GetHTMLView(writer io.Writer) error {
	resp, err := s.s3.GetObject(&s3.GetObjectInput{
		Bucket: s.bucket,
		Key:    s.key,
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == s3.ErrCodeNoSuchKey {
			s.logger.Print("View not found")
			return ErrNotFound
		}
		s.logger.Printf("Error getting file: %s", err.Error())
		return err
	}
	defer resp.Body.Close()

	// Read all in memory
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		s.logger.Printf("Error reading file: %s", err.Error())
		return err
	}

	view := fmt.Sprintf(htmlView, base64.StdEncoding.EncodeToString(content))

	if _, err := writer.Write([]byte(view)); err != nil {
		s.logger.Printf("Error writing view: %s", err.Error())
		return err
	}

	return nil
}

// SafePut overwrites the file if the new version is newer than the stored one.
func (s *store) SafePut(version time.Time, reader io.Reader) error {
	currentVersion, err := s.GetCurrentVersion()
	if err != nil {
		if err != ErrNotFound {
			return err
		}
		currentVersion = time.Time{}
	}

	if currentVersion.After(version) {
		s.logger.Printf("Version conflict, the stored version is newer")
		return ErrVersionConflict
	}

	return s.write(version, reader)
}

// Overwrite overwrites the version stored.
func (s *store) Overwrite(reader io.Reader) error {
	return s.write(time.Now(), reader)
}

func (s *store) write(version time.Time, reader io.Reader) error {
	if _, err := s.s3.PutObject(&s3.PutObjectInput{
		Body:            aws.ReadSeekCloser(reader),
		Bucket:          s.bucket,
		Key:             s.key,
		ContentEncoding: contentEncoding,
		ContentType:     contentType,
		Metadata:        map[string]*string{"version": aws.String(version.Format(time.RFC1123))},
	}); err != nil {
		s.logger.Printf("Can't store the file: %s", err.Error())
		return err
	}

	return nil
}
