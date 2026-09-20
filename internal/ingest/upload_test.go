package ingest

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func TestFileStashPreviewApply(t *testing.T) {
	s := NewFileStash()
	id := s.Put(FilePayload{Catalog: "dkbr", Filename: "t.csv", Data: []byte("id,name\n1,a\n")})
	if id == "" {
		t.Fatal("empty id")
	}
	p, err := FilePayload{PreviewID: id, Version: "BR25"}.Resolve(s)
	if err != nil {
		t.Fatal(err)
	}
	if p.Filename != "t.csv" || p.Version != "BR25" || p.Catalog != "dkbr" {
		t.Fatalf("%+v", p)
	}
	_, err = FilePayload{PreviewID: "nope"}.Resolve(s)
	if err != ErrPreviewGone {
		t.Fatalf("got %v", err)
	}
}

func TestReadMultipart(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("catalog", "dkbr")
	_ = w.WriteField("version", "BR18")
	_ = w.WriteField("map.id", "uid")
	fw, err := w.CreateFormFile("file", "x.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("uid,n\n1,a\n"))
	_ = w.Close()
	req := httptest.NewRequest("POST", "/ingest", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	p, err := ReadMultipart(req)
	if err != nil {
		t.Fatal(err)
	}
	if p.Catalog != "dkbr" || p.Version != "BR18" || p.Map["id"] != "uid" || p.Filename != "x.csv" {
		t.Fatalf("%+v", p)
	}
}
