package store

import (
	"errors"
	"io"
	"time"
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
        (function (d){
            var file = d.getElementById("file").textContent;
            d.getElementById("view").innerHTML = (new showdown.Converter()).makeHtml(atob(file));
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
