package ingest

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const MaxUpload = 32 << 20

var (
	ErrFileRequired = errors.New("file is required")
	ErrPreviewGone  = errors.New("preview expired; upload the file again")
)

// FilePayload is one multipart upload (file and/or preview_id).
type FilePayload struct {
	Catalog   string
	Version   string
	Filename  string
	Data      []byte
	Map       map[string]string
	PreviewID string
}

// ReadMultipart parses catalog, version, Column map, file, and preview_id.
func ReadMultipart(r *http.Request) (FilePayload, error) {
	if err := r.ParseMultipartForm(MaxUpload); err != nil {
		return FilePayload{}, ErrFileRequired
	}
	p := FilePayload{
		Catalog:   r.FormValue("catalog"),
		Version:   r.FormValue("version"),
		PreviewID: r.FormValue("preview_id"),
	}
	if r.MultipartForm != nil {
		p.Map = ParseMapping(r.MultipartForm.Value)
	}
	file, hdr, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, MaxUpload))
		if err != nil {
			return FilePayload{}, ErrFileRequired
		}
		p.Data = data
		if hdr != nil {
			p.Filename = hdr.Filename
		}
	}
	return p, nil
}

// Resolve uses a stashed preview when the file is missing.
func (p FilePayload) Resolve(s *FileStash) (FilePayload, error) {
	if len(p.Data) > 0 {
		return p, nil
	}
	if p.PreviewID == "" || s == nil {
		return p, ErrFileRequired
	}
	got, ok := s.Get(p.PreviewID)
	if !ok {
		return p, ErrPreviewGone
	}
	if p.Catalog != "" {
		got.Catalog = p.Catalog
	}
	if p.Version != "" {
		got.Version = p.Version
	}
	if len(p.Map) > 0 {
		got.Map = p.Map
	}
	got.PreviewID = p.PreviewID
	return got, nil
}

func (p FilePayload) Ingester() FileIngester {
	return FileIngester{
		Catalog: p.Catalog,
		Version: p.Version,
		Data:    p.Data,
		Name:    p.Filename,
		Map:     p.Map,
	}
}

func ResultBody(res Result) map[string]any {
	return map[string]any{
		"catalog":  res.Catalog,
		"origin":   res.Origin,
		"version":  res.Version,
		"seen":     res.Seen,
		"upserted": res.Upserted,
		"skipped":  res.Skipped,
	}
}

// FileStash holds preview bytes so Apply does not need a second upload.
type FileStash struct {
	mu    sync.Mutex
	items map[string]stashedFile
}

type stashedFile struct {
	p  FilePayload
	at time.Time
}

func NewFileStash() *FileStash {
	return &FileStash{items: map[string]stashedFile{}}
}

func (s *FileStash) Put(p FilePayload) string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	id := newPreviewID()
	p.PreviewID = id
	s.items[id] = stashedFile{p: p, at: time.Now()}
	return id
}

func (s *FileStash) Get(id string) (FilePayload, bool) {
	if s == nil || id == "" {
		return FilePayload{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	it, ok := s.items[id]
	if !ok {
		return FilePayload{}, false
	}
	return it.p, true
}

func (s *FileStash) gcLocked() {
	cut := time.Now().Add(-30 * time.Minute)
	for id, it := range s.items {
		if it.at.Before(cut) {
			delete(s.items, id)
		}
	}
	for len(s.items) > 8 {
		var oldest string
		var t time.Time
		for id, it := range s.items {
			if oldest == "" || it.at.Before(t) {
				oldest, t = id, it.at
			}
		}
		delete(s.items, oldest)
	}
}

func newPreviewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func (r *Runner) FileStash() *FileStash {
	if r == nil {
		return NewFileStash()
	}
	if r.Files == nil {
		r.Files = NewFileStash()
	}
	return r.Files
}
