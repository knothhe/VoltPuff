// Command icon regenerates the original application and menu bar icons.
package main

import (
	"image"
	"image/png"
	"log"
	"os"

	"github.com/knothhe/VoltPuff/internal/artwork"
)

func main() {
	if err := os.MkdirAll("assets", 0o755); err != nil {
		log.Fatal(err)
	}
	for path, img := range map[string]image.Image{"assets/icon.png": artwork.Icon(1024, false), "assets/tray.png": artwork.Tray()} {
		f, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			log.Fatal(err)
		}
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}
}
