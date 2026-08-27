package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Songxwn/rack-auto-ISO/internal/ipxescript"
	"github.com/Songxwn/rack-auto-ISO/internal/isogen"
	"github.com/Songxwn/rack-auto-ISO/internal/isoprep"
	"github.com/Songxwn/rack-auto-ISO/internal/model"
	"github.com/Songxwn/rack-auto-ISO/internal/store"
	"github.com/Songxwn/rack-auto-ISO/web"
	"github.com/google/uuid"
)

type Server struct {
	store   *store.Store
	version string
	assets  string
}

func New(st *store.Store, version string) *Server {
	execPath, _ := os.Executable()
	candidates := []string{
		os.Getenv("IPXE_ASSETS"),
		filepath.Join(st.DataDir(), "assets"),
		filepath.Join(filepath.Dir(execPath), "assets"),
		"assets",
		"internal/isogen/assets",
	}
	_ = os.MkdirAll(st.BootDir(), 0o755)
	return &Server{
		store:   st,
		version: version,
		assets:  isogen.ResolveAssetsDir(candidates...),
	}
}

func (s *Server) bootPaths() ipxescript.BootPaths {
	base := strings.TrimRight(s.store.Settings().PublicURL, "/")
	p := ipxescript.BootPaths{PublicBase: base}
	if base != "" {
		p.ISOBase = base + "/files/isos"
		p.BootBase = base + "/files/boot"
		p.AssetsBase = base + "/files/assets"
		p.Wimboot = base + "/files/assets/wimboot"
		p.Memdisk = base + "/files/assets/memdisk"
	}
	return p
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /", s.ui())
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(web.Static))))

	mux.HandleFunc("GET /boot.ipxe", s.serveBoot)
	mux.HandleFunc("GET /menu.ipxe", s.serveMenu)
	mux.HandleFunc("GET /embed.ipxe", s.serveEmbed)
	mux.Handle("GET /files/isos/", http.StripPrefix("/files/isos/", http.FileServer(http.Dir(s.store.ISODir()))))
	mux.Handle("GET /files/boot/", http.StripPrefix("/files/boot/", http.FileServer(http.Dir(s.store.BootDir()))))
	if s.assets != "" {
		mux.Handle("GET /files/assets/", http.StripPrefix("/files/assets/", http.FileServer(http.Dir(s.assets))))
	}

	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("PUT /api/settings", s.putSettings)
	mux.HandleFunc("GET /api/menus", s.listMenus)
	mux.HandleFunc("GET /api/menus/{id}", s.getMenu)
	mux.HandleFunc("POST /api/menus", s.createMenu)
	mux.HandleFunc("PUT /api/menus/{id}", s.updateMenu)
	mux.HandleFunc("DELETE /api/menus/{id}", s.deleteMenu)
	mux.HandleFunc("GET /api/menus/{id}/preview", s.previewMenu)
	mux.HandleFunc("GET /api/isos", s.listISOs)
	mux.HandleFunc("POST /api/isos", s.uploadISO)
	mux.HandleFunc("PUT /api/isos/{id}", s.updateISO)
	mux.HandleFunc("DELETE /api/isos/{id}", s.deleteISO)
	mux.HandleFunc("GET /api/iso/export", s.exportISO)
	mux.HandleFunc("GET /api/iso/bundle.zip", s.exportBundle)
	mux.HandleFunc("GET /api/iso/embed.ipxe", s.serveEmbed)

	return withCORS(mux)
}

func (s *Server) ui() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := web.Static.ReadFile("index.html")
		if err != nil {
			http.Error(w, "ui missing", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"ok":      true,
		"version": s.version,
		"assets":  s.assets != "",
		"wimboot": s.assets != "" && fileExists(filepath.Join(s.assets, "wimboot")),
		"memdisk": s.assets != "" && fileExists(filepath.Join(s.assets, "memdisk")),
	})
}

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.store.Settings())
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var cfg model.Settings
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.store.SaveSettings(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.store.Settings())
}

func (s *Server) listMenus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.store.ListMenus())
}

func (s *Server) getMenu(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.GetMenu(r.PathValue("id"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, m)
}

func (s *Server) createMenu(w http.ResponseWriter, r *http.Request) {
	var m model.Menu
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m.ID = ""
	out, err := s.store.UpsertMenu(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, out)
}

func (s *Server) updateMenu(w http.ResponseWriter, r *http.Request) {
	var m model.Menu
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m.ID = r.PathValue("id")
	out, err := s.store.UpsertMenu(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, out)
}

func (s *Server) deleteMenu(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteMenu(r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) previewMenu(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.GetMenu(r.PathValue("id"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	script := ipxescript.MenuScript(m, s.store.Settings(), s.store.ListISOs(), s.bootPaths())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(script))
}

func (s *Server) listISOs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.store.ListISOs())
}

func (s *Server) uploadISO(w http.ResponseWriter, r *http.Request) {
	const maxUpload = 16 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "invalid multipart: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = hdr.Filename
	}
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if ext == "" {
		ext = ".iso"
	}
	id := uuid.NewString()
	filename := id + ext
	dstPath := s.store.ISOPath(filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	written, err := io.Copy(dst, file)
	closeErr := dst.Close()
	if err != nil {
		_ = os.Remove(dstPath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if closeErr != nil {
		_ = os.Remove(dstPath)
		http.Error(w, closeErr.Error(), http.StatusInternalServerError)
		return
	}

	meta := model.ISOFile{
		ID:          id,
		Name:        name,
		Filename:    filename,
		Size:        written,
		ContentType: hdr.Header.Get("Content-Type"),
		Note:        r.FormValue("note"),
		Distro:      model.DistroGeneric,
		BootMethod:  "memdisk",
	}

	prep := isoprep.Prepare(dstPath, s.store.BootDir(), id)
	meta.Distro = prep.Distro
	meta.BootMethod = prep.BootMethod
	meta.PrepDir = prep.PrepDir
	if prep.Error != "" {
		meta.PrepError = prep.Error
		meta.PrepOK = false
		// Debian still boots via mirror params; keep method for UI clarity.
		if prep.Distro != model.DistroDebian {
			meta.BootMethod = "memdisk"
		}
	} else {
		meta.PrepOK = true
	}

	// Optional override: force memdisk for any distro (BIOS RAM boot).
	if strings.EqualFold(r.FormValue("bootMethod"), "memdisk") {
		meta.BootMethod = "memdisk"
	}

	saved, err := s.store.AddISO(meta)
	if err != nil {
		_ = os.Remove(dstPath)
		_ = os.RemoveAll(filepath.Join(s.store.BootDir(), id))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, saved)
}

func (s *Server) updateISO(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cur, err := s.store.GetISO(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var body struct {
		BootMethod string `json:"bootMethod"`
		Name       string `json:"name"`
		Note       string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.BootMethod != "" {
		switch body.BootMethod {
		case "memdisk", "sanboot", "debian-mirror", "kernel-repo", "esxi-mboot", "wimboot", "ubuntu-kernel":
			cur.BootMethod = body.BootMethod
		default:
			http.Error(w, "unsupported bootMethod", http.StatusBadRequest)
			return
		}
	}
	if body.Name != "" {
		cur.Name = body.Name
	}
	if body.Note != "" {
		cur.Note = body.Note
	}
	saved, err := s.store.UpdateISO(cur)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, saved)
}

func (s *Server) deleteISO(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteISO(r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) serveBoot(w http.ResponseWriter, r *http.Request) {
	menuID := r.URL.Query().Get("menu")
	script := ipxescript.BootScript(s.store.Settings(), menuID)
	writeIPXE(w, script)
}

func (s *Server) serveMenu(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		id = "default"
	}
	m, err := s.store.GetMenu(id)
	if err != nil {
		http.Error(w, "menu not found", http.StatusNotFound)
		return
	}
	script := ipxescript.MenuScript(m, s.store.Settings(), s.store.ListISOs(), s.bootPaths())
	writeIPXE(w, script)
}

func (s *Server) serveEmbed(w http.ResponseWriter, _ *http.Request) {
	script := ipxescript.EmbedScript(s.store.Settings())
	writeIPXE(w, script)
}

func (s *Server) exportISO(w http.ResponseWriter, r *http.Request) {
	script := ipxescript.EmbedScript(s.store.Settings())
	tmp, err := os.CreateTemp("", "ipxe-*.iso")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	if err := isogen.GenerateISO(script, s.assets, tmpPath); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="ipxe-custom.iso"`)
	http.ServeFile(w, r, tmpPath)
}

func (s *Server) exportBundle(w http.ResponseWriter, _ *http.Request) {
	script := ipxescript.EmbedScript(s.store.Settings())
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="ipxe-bundle.zip"`)
	if err := isogen.BundleZIP(script, s.assets, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func writeIPXE(w http.ResponseWriter, script string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(script))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
