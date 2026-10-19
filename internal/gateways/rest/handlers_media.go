package rest

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

const (
	mediaFormField = "file"
	// multipartOverhead is room for multipart boundaries, headers and other form fields.
	multipartOverhead = 64 << 10
)

type mediaView struct {
	ID          string `json:"id" example:"3f1d2c4b-8a9e-4b7c-9d2e-1a2b3c4d5e6f"`
	URL         string `json:"url" example:"/v1/media/3f1d2c4b-8a9e-4b7c-9d2e-1a2b3c4d5e6f"`
	ContentType string `json:"content_type" example:"image/jpeg"`
	Size        int64  `json:"size" example:"183244"` // bytes
}

// UploadMedia godoc swagger
// @Summary uploads an image (a profile photo) to S3
// @Description - send form data (multipart/form-data or application/x-www-form-urlencoded) with one field "file":
// @Description   either the file itself (multipart file part) or the file as a base64 string; a data URL
// @Description   prefix such as "data:image/png;base64," is allowed
// @Description - accepted: JPEG, PNG or WebP, detected from the bytes, not from the file name (422 otherwise)
// @Description - at most MEDIA_MAX_SIZE bytes after decoding, default 100 MiB (422 otherwise)
// @Description - the answer's id goes into users.photo_id (PATCH /v1/admin/users/{id} {"photo_id": id});
// @Description   url is where clients load the image from: GET /v1/media/{id} on this API
// @Description - 503 when S3 is not configured (S3_BUCKET empty)
// @Tags media
// @Accept multipart/form-data
// @Accept x-www-form-urlencoded
// @Produce json
// @Param file formData file true "the image: a file part, or a base64 string in the same field"
// @Success 200 {object} rest.R{data=rest.mediaView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 422 {object} rest.UnprocessableContentResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Failure 503 {object} rest.ServiceUnavailableResponse
// @Router /v1/media [POST]
func (s *Server) UploadMedia() gin.HandlerFunc {
	return s.uploadMedia(false)
}

// AdminUploadMedia godoc swagger
// @Summary uploads an image or a video (a daily video) to S3
// @Description - same form and limits as POST /v1/media; also accepts MP4 and WebM videos, detected from the bytes
// @Description - the answer's id goes into posts.video_id; GET /v1/media/{id} redirects to the file, video players follow it
// @Tags admin
// @Security AdminKey
// @Accept multipart/form-data
// @Accept x-www-form-urlencoded
// @Produce json
// @Param file formData file true "the image or video: a file part, or a base64 string in the same field"
// @Success 200 {object} rest.R{data=rest.mediaView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 422 {object} rest.UnprocessableContentResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Failure 503 {object} rest.ServiceUnavailableResponse
// @Router /v1/admin/media [POST]
func (s *Server) AdminUploadMedia() gin.HandlerFunc {
	return s.uploadMedia(true)
}

func (s *Server) uploadMedia(allowVideo bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mediaUploader == nil {
			fail(c, http.StatusServiceUnavailable, _errCodeUnavailable, "media uploads are disabled")

			return
		}

		maxSize := s.mediaUploader.MaxSize()
		// base64 is 4/3 the size of the file it encodes.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize/3*4+4+multipartOverhead)

		data, err := readMediaField(c, maxSize)
		if err != nil {
			Return(c, nil, err)

			return
		}

		m, err := s.mediaUploader.Execute(c.Request.Context(), domain.MediaUpload{Data: data, AllowVideo: allowVideo})
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, mediaView{ID: m.ID, URL: mediaPath(m.ID), ContentType: m.ContentType, Size: m.Size}, nil)
	}
}

// readMediaField returns the bytes of the "file" form field: a multipart file part, or a base64 string.
// It reads at most maxSize+1 bytes of a file part so the use case can reject a file that is too big.
func readMediaField(c *gin.Context, maxSize int64) ([]byte, error) {
	// Parse the form explicitly: c.FormFile hides the error of a urlencoded body (e.g. too large).
	var err error
	if c.ContentType() == gin.MIMEMultipartPOSTForm {
		err = c.Request.ParseMultipartForm(maxSize + multipartOverhead)
	} else {
		err = c.Request.ParseForm()
	}

	if err != nil {
		return nil, formError(err, maxSize)
	}

	if c.Request.MultipartForm != nil && len(c.Request.MultipartForm.File[mediaFormField]) > 0 {
		f, err := c.Request.MultipartForm.File[mediaFormField][0].Open()
		if err != nil {
			return nil, errs.Wrap(err)
		}
		defer f.Close()

		data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
		if err != nil {
			return nil, errs.Wrap(err)
		}

		return data, nil
	}

	value, ok := c.GetPostForm(mediaFormField)
	if !ok || strings.TrimSpace(value) == "" {
		return nil, errs.Errf(errs.ErrBadRequest, "form field %q is missing: send the file or its base64 string", mediaFormField)
	}

	data, err := decodeBase64(value)
	if err != nil {
		return nil, errs.Errf(errs.ErrBadRequest, "form field %q is not valid base64", mediaFormField)
	}

	return data, nil
}

func formError(err error, maxSize int64) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return errs.Errf(errs.ErrValidation, "request is too large, the file limit is %d bytes", maxSize)
	}

	return errs.Errf(errs.ErrBadRequest, "invalid form data: %s", err.Error())
}

// decodeBase64 accepts standard or URL-safe base64, padded or not, with an optional data URL prefix
// ("data:image/png;base64,") and line breaks.
func decodeBase64(s string) ([]byte, error) {
	if strings.HasPrefix(s, "data:") {
		if i := strings.Index(s, ","); i >= 0 {
			s = s[i+1:]
		}
	}

	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}

		return r
	}, s)

	// In x-www-form-urlencoded bodies an unescaped '+' arrives as ' ' and is dropped above;
	// clients must URL-encode the value (every HTTP client does for form data).
	var err error

	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		var data []byte
		if data, err = enc.DecodeString(s); err == nil {
			return data, nil
		}
	}

	return nil, err
}

// mediaPath is the API path that serves the media with this id (GET /v1/media/{id}).
func mediaPath(id string) string {
	return "/v1/media/" + id
}

// photoURL is mediaPath for a photo id, or nil without a photo.
func photoURL(id *string) *string {
	if id == nil || *id == "" {
		return nil
	}

	p := mediaPath(*id)

	return &p
}

// GetMedia godoc swagger
// @Summary opens an uploaded image
// @Description - redirects (302) to a short-lived signed link to the image in the private S3 bucket
// @Description   (valid MEDIA_LINK_TTL, default 1 hour); image widgets follow it on their own
// @Description - needs no token, so <img> and Image.network work; ids are random UUIDs
// @Description - unknown id: 404; S3 not configured: 503
// @Tags media
// @Param id path string true "media id (photo_id)"
// @Success 302
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Failure 503 {object} rest.ServiceUnavailableResponse
// @Router /v1/media/{id} [GET]
func (s *Server) GetMedia() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.mediaLinker == nil {
			fail(c, http.StatusServiceUnavailable, _errCodeUnavailable, "media is disabled")

			return
		}

		link, err := s.mediaLinker.Link(c.Request.Context(), c.Param("id"))
		if err != nil {
			Return(c, nil, err)

			return
		}

		// The signed link expires, so clients must not cache the redirect longer than it lives.
		c.Header("Cache-Control", "private, max-age=300")
		c.Redirect(http.StatusFound, link)
	}
}
