package app

import (
	"compress/gzip"
	"io"
	"net/http"
	"slices"
	"strings"
)

func gzipMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptEncodinng := strings.Split(r.Header.Get("Accept-Encoding"), ",")
		gzipOut := slices.Index(acceptEncodinng, "gzip") != -1
		if gzipOut {
			// оборачиваем writer
			cw := newCompressWriter(w)
			defer cw.Close()
			w = cw
		}

		contentEncoding := strings.Split(r.Header.Get("Content-Encoding"), ", ")
		gzipIn := slices.Index(contentEncoding, "gzip") != -1
		if gzipIn {
			// оборачиваем тело запроса в io.Reader с поддержкой декомпрессии
			cr, err := newCompressReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer cr.Close()
			// меняем тело запроса на новое
			r.Body = cr
		}

		h.ServeHTTP(w, r)
	})
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

type compressWriter struct {
	w  http.ResponseWriter
	zw *gzip.Writer
}

func (w *compressWriter) Header() http.Header {
	return w.w.Header()
}

func (w *compressWriter) Write(buf []byte) (int, error) {
	return w.w.Write(buf)
}

func (w *compressWriter) WriteHeader(statusCode int) {
	if statusCode < 300 && statusCode >= 200 {
		w.w.Header().Set("Content-Encoding", "gzip")
	}
	w.w.WriteHeader(statusCode)
}

func (w *compressWriter) Close() error {
	return w.zw.Close()
}

// compressReader реализует интерфейс io.ReadCloser и позволяет прозрачно для сервера
// декомпрессировать получаемые от клиента данные
type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

func (r *compressReader) Close() error {
	if err := r.r.Close(); err != nil {
		return err
	}
	return r.zr.Close()
}

func (r *compressReader) Read(p []byte) (n int, err error) {
	return r.zr.Read(p)
}
