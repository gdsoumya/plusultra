package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepShrinksImagesAndKeepsTheDataset(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name string, data []byte) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	add("free-exercise-db-abc/dist/exercises.json", []byte("[]"))
	add("free-exercise-db-abc/README.md", []byte("skip"))
	var img bytes.Buffer
	tall := image.NewRGBA(image.Rect(0, 0, 500, 1000))
	for i := range tall.Pix {
		tall.Pix[i] = 0x80
	}
	if err := jpeg.Encode(&img, tall, nil); err != nil {
		t.Fatal(err)
	}
	add("free-exercise-db-abc/exercises/Squat/0.jpg", img.Bytes())
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "fed.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	if err := prep(zipPath, out); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(out, "dist", "exercises.json")); err != nil || string(data) != "[]" { //nolint:gosec // test temp dir
		t.Fatalf("exercises.json = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(out, "README.md")); err == nil {
		t.Fatal("README.md was copied")
	}
	f, err := os.Open(filepath.Join(out, "exercises", "Squat", "0.jpg")) //nolint:gosec // test temp dir
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // test
	got, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := got.Bounds(); b.Dx() != 320 || b.Dy() != 640 {
		t.Fatalf("shrunk to %v, want 320x640", b)
	}
	if r, _, _, _ := got.At(160, 320).RGBA(); r>>8 < 0x78 || r>>8 > 0x88 {
		t.Fatalf("centre pixel red = %#x, want about 0x80", r>>8)
	}
	if small := downscale(image.NewRGBA(image.Rect(0, 0, 10, 10)), 800); small.Bounds().Dx() != 10 {
		t.Fatal("small images must come back as they are")
	}
}
