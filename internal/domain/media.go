package domain

import "time"

// Media is an uploaded file stored in object storage (S3). URL is built from Key at read time.
type Media struct {
	ID          string
	Key         string
	URL         string
	ContentType string
	Size        int64
	CreatedAt   time.Time
}

// MediaUpload is a file to store, already decoded from the request.
type MediaUpload struct {
	Data []byte
	// AllowVideo also accepts the video types (admin uploads of daily videos).
	AllowVideo bool
}

// imageExtensions maps the accepted image types, as sniffed from the content, to file extensions.
var imageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// videoExtensions maps the accepted video types, as sniffed from the content, to file extensions.
var videoExtensions = map[string]string{
	"video/mp4":  ".mp4",
	"video/webm": ".webm",
}

// ImageExtension returns the file extension for an accepted image content type.
func ImageExtension(contentType string) (string, bool) {
	ext, ok := imageExtensions[contentType]

	return ext, ok
}

// VideoExtension returns the file extension for an accepted video content type.
func VideoExtension(contentType string) (string, bool) {
	ext, ok := videoExtensions[contentType]

	return ext, ok
}

// IsVideo reports whether contentType is an accepted video type.
func IsVideo(contentType string) bool {
	_, ok := videoExtensions[contentType]

	return ok
}
