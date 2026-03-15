package storage

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// NewLocalHandler returns an http.Handler that handles file uploads and serving
// for LocalStorage. Mount it at the baseURL path (e.g. /uploads/files).
//
// Routes:
//
//	PUT  /put/{token}  — upload a file using a token from GenerateUploadURL
//	GET  /{key...}     — serve a stored file
func NewLocalHandler(ls *LocalStorage) http.Handler {
	r := chi.NewRouter()
	r.Put("/put/{token}", uploadHandler(ls))
	r.Get("/*", serveHandler(ls))
	return r
}

func uploadHandler(ls *LocalStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		if token == "" {
			http.Error(w, `{"error":"missing token"}`, http.StatusBadRequest)
			return
		}

		params, err := ls.ConsumePendingUpload(token)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired upload token"}`, http.StatusBadRequest)
			return
		}

		// Limit the request body to the declared max size + 1 byte to detect oversized bodies.
		body := http.MaxBytesReader(w, r.Body, params.MaxSize)
		defer body.Close()

		dest := ls.FilePath(params.Key)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		f, err := os.Create(dest)
		if err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if _, err := io.Copy(f, body); err != nil {
			os.Remove(dest)
			if isMaxBytesError(err) {
				http.Error(w, `{"error":"file too large"}`, http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func serveHandler(ls *LocalStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "*")
		if key == "" || strings.Contains(key, "..") {
			http.NotFound(w, r)
			return
		}
		path := ls.FilePath(key)
		http.ServeFile(w, r, path)
	}
}

func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}
