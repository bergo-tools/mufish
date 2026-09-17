package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler 提供嵌入的前端静态资源，并对未知路径回退到 index.html。
func (h *Handler) spaHandler() http.Handler {
	fileServer := http.FileServer(http.FS(h.assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeErr(w, http.StatusNotFound, errNotFound())
			return
		}
		upath := path.Clean("/" + r.URL.Path)
		rel := strings.TrimPrefix(upath, "/")
		if rel == "" {
			rel = "index.html"
		}
		if f, err := h.assets.Open(rel); err == nil {
			f.Close()
			// 带 hash 的静态资源长缓存
			if strings.HasPrefix(rel, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=86400")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		serveIndex(w, r, h.assets)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	data, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("前端资源缺失，请先执行前端构建（make web）"))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

func errNotFound() error { return errString("接口不存在") }

type errString string

func (e errString) Error() string { return string(e) }
