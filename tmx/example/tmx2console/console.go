/*
   Copyright (c) Utkan Güngördü <utkan@freeconsole.org>

   This program is free software; you can redistribute it and/or modify
   it under the terms of the GNU General Public License as
   published by the Free Software Foundation; either version 3 or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of

   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the

   GNU General Public License for more details


   You should have received a copy of the GNU General Public
   License along with this program; if not, write to the
   Free Software Foundation, Inc.,
   51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
*/

package main

import (
	"encoding/binary"
	"errors"
	"github.com/salviati/gbacomp"
	"github.com/salviati/go-tmx/tmx"
	"io"
	"os"
	"path/filepath"
)

var (
	CompressionMethods = map[string]gbacomp.Method{"LZ77": gbacomp.LZ77, "RLE": gbacomp.RLE, "Huffman4": gbacomp.Huffman4, "Huffman8": gbacomp.Huffman8}
)

var (
	WrongFileExtension       = errors.New("The file extension must be .tmx")
	InvalidCompressionMethod = errors.New("Invalid compression method")
	TooManyTiles             = errors.New("Too many tiles in the tileset")
)

var (
	MultipleTilesets = errors.New("tmx: a layer must use tile from only one tileset.")
	EmptyLayer       = errors.New("tmx: layer is empty; tileset cannot be determined.")
)

const (
	TMXExt = ".tmx"
)

type Console interface {
	MaxTiles(m *tmx.Map, l *tmx.Layer) int                                                 // Maximum number of allowed tiles
	ScreenblockEntry(m *tmx.Map, l *tmx.Layer, tile *tmx.DecodedTile) (interface{}, error) // Should convert a GID to machine-specific screenblock entry.
	ByteOrder() binary.ByteOrder
}

// Converts a tmx file to a console-specific format. Output is written in files.
func Do(c Console, filename string) error {
	r, err := os.Open(filename)
	if err != nil {
		return err
	}

	m, err := tmx.Read(r)
	if err != nil {
		return err
	}

	ext := filepath.Ext(filename)
	if ext != TMXExt {
		return WrongFileExtension
	}
	filenameBare := filename[:len(filename)-len(ext)-1]

	saveLayer := func(l *tmx.Layer, w io.WriteCloser) error {
		defer w.Close()
		b := w

		if l.Tileset == nil {
			if l.Empty {
				return EmptyLayer
			} else {
				return MultipleTilesets
			}
		}

		if len(l.Tileset.Tiles) > c.MaxTiles(m, l) {
			return TooManyTiles
		}

		compression, _ := GetProperty(l.Properties, "Compression")
		if compression != "" {
			compressionMethod, ok := CompressionMethods[compression]
			if !ok {
				return InvalidCompressionMethod
			}
			b = gbacomp.NewCompressor(w, compressionMethod)
			defer b.Close()
		}

		i := 0
		for y := 0; y < m.Height; y++ {
			for x := 0; x < m.Width; x++ {
				tile, err := c.ScreenblockEntry(m, l, l.DecodedTiles[i])
				if err != nil {
					return err
				}

				err = binary.Write(b, c.ByteOrder(), tile)

				i++
			}
		}
		return nil
	}

	saveLayerBitmap := func(l *tmx.Layer, w io.WriteCloser) error {
		defer w.Close()

		i := uint(0)
		var d uint8
		for y := 0; y < m.Height; y++ {
			for x := 0; x < m.Width; x++ {
				if l.DecodedTiles[i].Nil == false {
					d |= 1 << i
				}
				i++
				if i&7 == 0 {
					_, err = w.Write([]byte{d})
					if err != nil {
						return err
					}

					i = 0
				}
			}
		}
		return nil
	}

	for i := 0; i < len(m.Layers); i++ {
		l := &m.Layers[i]

		bitmap, err := GetProperty(l.Properties, "Bitmap")

		name := filenameBare + "." + l.Name + ".layer"
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			return err
		}
		defer f.Close()

		if bitmap == "true" {
			if err := saveLayerBitmap(l, f); err != nil {
				return err
			}
		} else {
			if err := saveLayer(l, f); err != nil {
				return err
			}
		}
	}

	return nil
}
