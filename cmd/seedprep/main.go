// Command seedprep unpacks a free-exercise-db archive into the checkout layout
// the importer reads, re-encoding every image to at most 640 px on its long
// side at JPEG quality 60: the images decide the Docker image size.
package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	maxSide = 640
	quality = 60
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: seedprep <free-exercise-db zip> <out dir>")
		os.Exit(2)
	}
	if err := prep(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// prep writes out/dist/exercises.json and out/exercises/<name>/<n>.jpg from the
// archive, whose entries sit under one top-level folder.
func prep(zipPath, out string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close() //nolint:errcheck // read-only

	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	errs := make(chan error, len(zr.File))
	for _, f := range zr.File {
		_, rel, _ := strings.Cut(f.Name, "/")
		shrink := strings.HasPrefix(rel, "exercises/") && strings.HasSuffix(rel, ".jpg")
		if !shrink && rel != "dist/exercises.json" || !filepath.IsLocal(rel) {
			continue
		}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := write(f, filepath.Join(out, filepath.FromSlash(rel)), shrink); err != nil {
				errs <- fmt.Errorf("%s: %w", rel, err)
			}
		})
	}
	wg.Wait()
	close(errs)
	var all []error
	for err := range errs {
		all = append(all, err)
	}
	return errors.Join(all...)
}

func write(f *zip.File, dst string, shrink bool) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()                                              //nolint:errcheck // read-only
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil { //nolint:gosec // rel passed IsLocal
		return err
	}
	w, err := os.Create(dst) //nolint:gosec // rel passed IsLocal
	if err != nil {
		return err
	}
	defer w.Close() //nolint:errcheck // closed again below for the error
	if !shrink {
		_, err = io.Copy(w, rc) //nolint:gosec // the archive is pinned by checksum in the Dockerfile
	} else {
		var img image.Image
		if img, err = jpeg.Decode(rc); err == nil {
			err = jpeg.Encode(w, downscale(img, maxSide), &jpeg.Options{Quality: quality})
		}
	}
	if err != nil {
		return err
	}
	return w.Close()
}

// downscale averages source pixels into a copy whose long side is at most
// maxSide; images already small enough come back as they are.
func downscale(src image.Image, maxSide int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	long := max(sw, sh)
	if long <= maxSide {
		return src
	}
	dw, dh := sw*maxSide/long, sh*maxSide/long
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		y0, y1 := y*sh/dh, max((y+1)*sh/dh, y*sh/dh+1)
		for x := range dw {
			x0, x1 := x*sw/dw, max((x+1)*sw/dw, x*sw/dw+1)
			var r, g, bl, n uint32
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, _ := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
					r, g, bl, n = r+cr, g+cg, bl+cb, n+1
				}
			}
			dst.Set(x, y, color.RGBA64{R: uint16(r / n), G: uint16(g / n), B: uint16(bl / n), A: 0xffff}) //nolint:gosec // averages of uint16 values
		}
	}
	return dst
}
