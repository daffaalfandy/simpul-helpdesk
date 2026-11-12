package httpapi

import (
	"bytes"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
)

// spaHandler serves the built web app. Real files are served as-is (hashed
// assets are cached forever); every other path gets index.html so client-side
// routes work. Unknown /api paths get a JSON 404, never the app.
func spaHandler(webFS fs.FS, log *slog.Logger) http.Handler {
	var index []byte
	if webFS != nil {
		index, _ = fs.ReadFile(webFS, spaIndex)
	}
	if index == nil {
		log.Warn("web app not built: run `make build` (in development use the Vite server)")
	}
	files := http.FileServerFS(webFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			writeAppError(w, r, apperr.NotFound())
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeAppError(w, r, apperr.MethodNotAllowed())
			return
		}
		if index == nil {
			http.Error(w, spaNotBuiltMsg, http.StatusNotFound)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != spaIndex {
			if st, err := fs.Stat(webFS, name); err == nil && !st.IsDir() {
				if strings.HasPrefix(name, spaAssetsPrefix) {
					w.Header().Set(headerCacheControl, cacheImmutable)
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set(headerCacheControl, cacheNoCache)
		w.Header().Set(headerContentType, contentTypeHTML)
		http.ServeContent(w, r, spaIndex, time.Time{}, bytes.NewReader(index))
	})
}
