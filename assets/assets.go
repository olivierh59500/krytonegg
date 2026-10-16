// Package assets embeds only data recovered from the supplied Amiga disk.
// The original executable is never linked to or executed by the Go port.
package assets

import (
	"embed"
	"fmt"
	"image"
	_ "image/png"
	"io/fs"
)

// Files contains the original level table, converted images, and tracker music.
//
//go:embed levels.json combat.json manifest.json presentation.json images sprites audio original/intro original/halloffame original/level.tab original/menu.art original/copyrigh.txt original/fame.art original/zz_3.bmp original/final.bmp
var Files embed.FS

// Read returns an embedded asset without depending on the working directory.
func Read(name string) ([]byte, error) {
	data, err := Files.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read original asset %q: %w", name, err)
	}
	return data, nil
}

// Image decodes a lossless conversion of the original planar artwork.
func Image(name string) (image.Image, error) {
	f, err := Files.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open original image %q: %w", name, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode original image %q: %w", name, err)
	}
	return img, nil
}

// Names lists the embedded assets for provenance checks and diagnostics.
func Names() ([]string, error) {
	var names []string
	err := fs.WalkDir(Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	return names, err
}
