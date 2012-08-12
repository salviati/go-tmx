/*
  Converts a TMX file to files which can be loaded to GBA.

  The TMX is assumed to have only one tileset.
  
  Data for each layer will be written in separate files. If the map "hello.tmx" has two layers "BG2" and "BG3",
  the data will be written into "hello.BG1.layer" "hello.BG2.layer".
  Depending on the Properties field of each layer, an encoding will take place:
  
  Bitmap=true: The layer will be encoded into a 1-bit-per-tile stream. NilTiles will be encoded as 0, others as 1.
  Useful for generating obstruction layer data in a compact form.
  
  NilTile=ID: NilTile is ordinarily encoded into NTiles by default (that is just out of the valid range of tiles).
  This property will override the default.
  
  Compression=Method where method is one of LZ77, RLE, Huffman4, Huffman8. Will compress the layer data.
  (Huffman4 is not implemented yet)
  
  The size of a layer file is MapWidth*MapHeight*2 byte for layers when Bitmap=true is not set.
  Size of a bitmap layer is MapWidth*MapHeight/8
*/
package main

import (
	"encoding/binary"
	"flag"
	"github.com/salviati/go-tmx/tmx"
	"github.com/salviati/gbacomp"
	"path/filepath"
	"log"
	"os"
	"io"
	"errors"
	"strconv"
)

const (
	HFlip = 1 << 10
	VFlip = 1 << 11
	Magic = "GBAMAP01"
	TMXExt = ".tmx"
)

var (
	InvalidNumberOfTilesets = errors.New("The number of tilesets used in a map should exactly be 1.")
	WrongFileExtension = errors.New("The file extension must be .tmx")
	PropertyNotUnique = errors.New("Layer Property is not unique")
	PropertyUnavailable = errors.New("Property does not exist")
	InvalidCompressionMethod = errors.New("Invalid compression method")

	CompressionMethods = map[string]gbacomp.Method{"LZ77": gbacomp.LZ77, "RLE": gbacomp.RLE, /*"Huffman4":gbacomp.Huffman4,*/ "Huffman8":gbacomp.Huffman8 }
)

func doTMX(filename string) error {
	m, err := tmx.NewMap(filename)
	if err != nil {
		return err
	}

	if len(m.Tilesets) != 1 {
		return InvalidNumberOfTilesets
	}
	
	ext := filepath.Ext(filename)
	if ext != TMXExt {
		return WrongFileExtension
	}
	filenameBare := filename[:len(filename)-len(ext)-1]



	nilTileDefault := uint16(len(m.Tilesets[0].Tiles))
	
	getProp := func (p *tmx.Properties, name string) (value string, err error) {
		values := p.Get(name)
		if len(values) > 1 { err = PropertyNotUnique; return }
		if len(value) == 0 { err = PropertyUnavailable; return}
		value = values[0]
		return
	}
	
	save := func(l *tmx.Layer, w io.WriteCloser) error {
		defer w.Close()
		b := w
		
		nilTile := nilTileDefault
		nilTileString, _ := getProp(&l.Properties, "NilTile")
		nilTileNew, err := strconv.ParseUint(nilTileString,10,16)
		if err == nil {
			nilTile = uint16(nilTileNew)
		}

		compression, _ := getProp(&l.Properties,"Compression")
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
				if l.DecodedTiles[i] == tmx.NilTile {
					err = binary.Write(b, binary.LittleEndian, nilTile)
					if err != nil {
						return err
					}
					continue
				}

				tid := uint16(l.DecodedTiles[i] & tmx.GIDMask)
				if l.DecodedTiles[i]&tmx.GIDHorizontalFlip != 0 {
					tid |= HFlip
				}
				if l.DecodedTiles[i]&tmx.GIDVerticalFlip != 0 {
					tid |= VFlip
				}
				// TODO(utkan); palette bank
				err = binary.Write(b, binary.LittleEndian, tid)
				if err != nil {
					return err
				}

				i++
			}
		}
		return nil
	}
	
	saveBitmap := func(l *tmx.Layer, w io.WriteCloser) error {
		defer w.Close()

		i := uint(0)
		var d uint8
		for y := 0; y < m.Height; y++ {
			for x := 0; x < m.Width; x++ {
				if l.DecodedTiles[i] != tmx.NilTile {
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

	for i:=0; i<len(m.Layers); i++ {
		l := &m.Layers[i]
		
		bitmap, err := getProp(&l.Properties,"Bitmap")
		
		name := filenameBare+"." + l.Name + ".layer"
		f, err := os.OpenFile(name, os.O_WRONLY, 0666)
		if err != nil {
			return err
		}
		defer f.Close()
		
		if bitmap == "true" {
			if err := saveBitmap(l, f); err != nil {
				return err
			}
		} else {
			if err := save(l, f); err != nil {
				return err
			}
		}
	}


	return nil
}

func main() {
	flag.Parse()

	for _, filename := range flag.Args() {
		if err := doTMX(filename); err != nil {
			log.Println(err)
		}
	}
}
