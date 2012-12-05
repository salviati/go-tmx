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

/*
  Converts a TMX file to files which can be loaded to GBA.

  The TMX is assumed to have only one tileset for now.

  Data for each layer will be written in separate files. If the map "hello.tmx" has two layers "BG2" and "BG3",
  the data will be written into "hello.BG1.layer" "hello.BG2.layer".

  You must use tiles from only one tileset in a layer.

  Layer Properties (GBA):

    Bitmap=true: The layer will be encoded into a 1-bit-per-tile stream. NilTiles will be encoded as 0, others as 1.
    Useful for generating obstruction layer data in a compact form.

    NilTile=ID: NilTile is ordinarily encoded into NTiles by default (that is just out of the valid range of tiles).
    This property will override the default.

    Compression=Method where method is one of LZ77, RLE, Huffman4, Huffman8. Will compress the layer data.

    Affine=true: Exported tile-data will become 8-bits per tile; flip bits will be discarded.

    BG=X where X can be 0,1,2 or 3. This will appear in the .map file, as a note to which hardware BG this layer corresponds to.

    The size of a layer file is MapWidth*MapHeight*2 byte for normal layers and MapWidth*MapHeight for affine layers.
    When Bitmap=true is set, however, it is MapWidth*MapHeight/8.

  Map File:
    Width, Height, filenames of all that is involved. # of tiles and BPP for each tileset.
*/
package main

/*
  TODO(utkan): Add Huffman1 for the sake of obstruction layer
  TODO(utkan): Process tilesets as well; export image and palette data. Options: compression, tile-reduction (unused/duplicate/flipped).
  TODO(utkan): Add a .map file that will completely describe how to load the whole map.
  TODO(utkan): Long-term: NES, SNES, Mega Drive, etc.
*/

import (
	"flag"
	"log"
)

var (
	consoleName = flag.String("console", "gba", "Name of the target console (can be one of: gba)")
	consoles    = map[string]Console{"gba": new(GBA)}
)

func getConsole(name string) Console {
	c, ok := consoles[name]
	if !ok {
		log.Fatal("No such console", name)
	}
	return c
}

func main() {
	flag.Parse()

	c := getConsole(*consoleName)

	for _, filename := range flag.Args() {
		if err := Do(c, filename); err != nil {
			log.Println(err)
		}
	}
}
