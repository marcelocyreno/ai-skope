package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ai-skope/aiss/internal/files"
	"github.com/ai-skope/aiss/internal/store"
)

// fileDTO is how a file reaches the picker: the real path for later reads,
// and the ~-abbreviated form to show.
type fileDTO struct {
	Path    string `json:"path"`
	Display string `json:"display"`
	Dir     string `json:"dir"`
	Name    string `json:"name"`
	Ext     string `json:"ext,omitempty"`
	Size    int64  `json:"size"`
	MTime   int64  `json:"mtime"`
	IsDir   bool   `json:"isDir"`
	Snippet string `json:"snippet,omitempty"`
}

func toFileDTO(f store.File) fileDTO {
	return fileDTO{
		Path: f.Path, Display: files.Tilde(f.Path), Dir: files.Tilde(filepath.Dir(f.Path)),
		Name: f.Name, Ext: f.Ext, Size: f.Size, MTime: f.MTime, IsDir: f.IsDir, Snippet: f.Snippet,
	}
}

// ---- folders ----

func (s *Server) listFolders(w http.ResponseWriter, r *http.Request) {
	list, err := s.DB.Folders()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, f := range list {
		out = append(out, map[string]any{
			"id": f.ID, "path": f.Path, "display": files.Tilde(f.Path), "access": f.Access,
			"fileCount": f.FileCount, "lastIndexedAt": f.LastIndexedAt, "createdAt": f.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"folders": out})
}

func (s *Server) createFolder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Path   string `json:"path"`
		Access string `json:"access"`
	}
	if !decode(w, r, &in) {
		return
	}
	abs, err := files.Expand(in.Path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "That path could not be read: "+err.Error())
		return
	}
	fi, err := os.Stat(abs)
	if err != nil || !fi.IsDir() {
		writeErr(w, http.StatusBadRequest, "not_a_folder", files.Tilde(abs)+" is not a folder on this computer.")
		return
	}
	f, err := s.DB.AddFolder(abs, in.Access)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	s.reindexInBackground(f)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": f.ID, "path": f.Path, "display": files.Tilde(f.Path), "access": f.Access,
	})
}

func (s *Server) patchFolder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Access string `json:"access"`
	}
	if !decode(w, r, &in) {
		return
	}
	id := r.PathValue("id")
	if err := s.DB.UpdateFolderAccess(id, in.Access); err != nil {
		writeStoreErr(w, err)
		return
	}
	if s.Watcher != nil {
		s.Watcher.Sync()
	}
	f, err := s.DB.Folder(id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": f.ID, "path": f.Path, "access": f.Access})
}

func (s *Server) deleteFolder(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.DeleteFolder(r.PathValue("id")); err != nil {
		writeStoreErr(w, err)
		return
	}
	if s.Watcher != nil {
		s.Watcher.Sync()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reindexFolder(w http.ResponseWriter, r *http.Request) {
	f, err := s.DB.Folder(r.PathValue("id"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	s.reindexInBackground(f)
	writeJSON(w, http.StatusAccepted, map[string]any{"folderId": f.ID, "state": "started"})
}

// reindexInBackground walks a folder without holding the request open;
// progress reaches the settings page over /v1/events.
func (s *Server) reindexInBackground(f store.Folder) {
	if s.Indexer == nil {
		return
	}
	go func() {
		_ = s.Indexer.IndexFolder(context.Background(), f)
		if s.Watcher != nil {
			s.Watcher.Sync()
		}
	}()
}

// ---- files ----

func (s *Server) searchFiles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	hits, err := s.DB.SearchFiles(q, intParam(r, "limit", 50))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := make([]fileDTO, 0, len(hits))
	for _, f := range hits {
		out = append(out, toFileDTO(f))
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": out, "query": q})
}

func (s *Server) recentFiles(w http.ResponseWriter, r *http.Request) {
	hits, err := s.DB.RecentFiles(intParam(r, "limit", 20))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := make([]fileDTO, 0, len(hits))
	for _, f := range hits {
		out = append(out, toFileDTO(f))
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": out})
}

func (s *Server) browseFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if strings.TrimSpace(path) == "" {
		// With no path, list the allowed folders themselves.
		s.listFolders(w, r)
		return
	}
	entries, err := s.Guard.Browse(path)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	out := make([]fileDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, fileDTO{
			Path: e.Path, Display: e.Display, Dir: files.Tilde(filepath.Dir(e.Path)),
			Name: e.Name, Ext: e.Ext, Size: e.Size, MTime: e.MTime, IsDir: e.IsDir,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out, "path": files.Tilde(path)})
}

func (s *Server) readFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if strings.TrimSpace(path) == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "Which file should I read?")
		return
	}
	c, err := s.Guard.Read(path)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	_ = s.DB.TouchRecentFile(c.Path)
	writeJSON(w, http.StatusOK, c)
}

// resolveFile turns a file:// URL from the browser into an allowed path, so
// the extension can ask about a local page it has open.
func (s *Server) resolveFile(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL string `json:"url"`
	}
	if !decode(w, r, &in) {
		return
	}
	path, folder, err := s.Guard.ResolveFileURL(in.URL)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path": path, "display": files.Tilde(path), "folderId": folder.ID,
	})
}
