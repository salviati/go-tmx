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

// A Go library that reads Tiled's TMX files
package tmx

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
)

const (
	GID_HORIZONTAL_FLIP = 0x80000000
	GID_VERTICAL_FLIP   = 0x40000000
	GID_DIAGONAL_FLIP   = 0x20000000
	GID_FLIP            = GID_HORIZONTAL_FLIP | GID_VERTICAL_FLIP | GID_DIAGONAL_FLIP
	NIL_TILE            = 0xffffffff // Beware of the nil tile! Can crash your game if not handled properly.
)

var (
	UnknownEncoding       = errors.New("tmx: invalid encoding scheme")
	UnknownCompression    = errors.New("tmx: invalid compression method")
	InvalidDecodedDataLen = errors.New("tmx: invalid decoded data length")
	InvalidGID            = errors.New("tmx: invalid GID")
)

type Map struct {
	Version      string        `xml:"title,attr"`
	Orientation  string        `xml:"orientation,attr"`
	Width        int           `xml:"width,attr"`
	Height       int           `xml:"height,attr"`
	TileWidth    int           `xml:"tilewidth,attr"`
	TileHeight   int           `xml:"tileheight,attr"`
	Properties   Properties    `xml:"properties"`
	Tilesets     []Tileset     `xml:"tileset"`
	Layers       []Layer       `xml:"layer"`
	ObjectGroups []ObjectGroup `xml:"objectgroup"`
}

type Tileset struct {
	FirstGID   uint32     `xml:"firstgid,attr"`
	Source     string     `xml:"source,attr"`
	Name       string     `xml:"name,attr"`
	TileWidth  int        `xml:"tilewidth,attr"`
	TileHeight int        `xml:"tileheight,attr"`
	Spacing    int        `xml:"spacing,attr"`
	Margin     int        `xml:"margin,attr"`
	Properties Properties `xml:"properties"`
	Image      Image      `xml:"image"`
	Tiles      []Tile     `xml:"tile"`
}

type Image struct {
	Source string `xml:"source,attr"`
	Trans  string `xml:"trans,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
}

type Tile struct {
	ID    int   `xml:"id,attr"`
	Image Image `xml:"image"`
}

type Layer struct {
	Name         string     `xml:"name,attr"`
	Opacity      float32    `xml:"opacity,attr"`
	Visible      bool       `xml:"visible,attr"`
	Properties   Properties `xml:"properties"`
	Data         Data       `xml:"data"`
	DecodedTiles []uint32   // This is probably the one you'd like to use, not data. Tile index at (x,y) is l.DecodedTiles[y*map.Width+x] &^ GID_FLIP (upper 3 bits indicate H/V/D flips).
}

type Data struct {
	Encoding    string     `xml:"encoding,attr"`
	Compression string     `xml:"compression,attr"`
	RawData     []byte     `xml:",innerxml"`
	DataTiles   []DataTile `xml:"tile"` // Only used when layer encoding is xml
}

type ObjectGroup struct {
	Name       string     `xml:"name,attr"`
	Color      string     `xml:"color,attr"`
	Opacity    float32    `xml:"opacity,attr"`
	Visible    bool       `xml:"visible,attr"`
	Properties Properties `xml:"properties"`
	Objects    []Object   `xml:"object"`
}

type Object struct {
	Name      string     `xml:"name,attr"`
	Type      string     `xml:"type,attr"`
	X         int        `xml:"x,attr"`
	Y         int        `xml:y",attr"`
	Width     int        `xml:"widrg,attr"`
	Height    int        `xml:"height,attr"`
	GID       int        `xml:"gid,attr"`
	Visible   bool       `xml:"visible,attr"`
	Polygons  []Polygon  `xml:"polygon"`
	PolyLines []PolyLine `xml:"polyline"`
}

type Polygon struct {
	Points string `xml:"points,attr"`
}

type PolyLine struct {
	Points string `xml:"points,attr"`
}

type Properties struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func (d *Data) decodeBase64() (data []byte, err error) {
	rawData := bytes.TrimSpace(d.RawData)
	r := bytes.NewReader(rawData)

	encr := base64.NewDecoder(base64.StdEncoding, r)

	var comr io.Reader
	switch d.Compression {
	case "gzip":
		comr, err = gzip.NewReader(encr)
		if err != nil {
			return
		}
	case "zlib":
		comr, err = zlib.NewReader(encr)
		if err != nil {
			return
		}
	case "":
		comr = encr
	default:
		err = UnknownCompression
		return
	}

	return ioutil.ReadAll(comr)
}

func (d *Data) decodeCSV() (data []uint32, err error) {
	cleaner := func(r rune) rune {
		if (r >= '0' && r <= '9') || r == ',' {
			return r
		}
		return -1
	}
	rawDataClean := strings.Map(cleaner, string(d.RawData))

	str := strings.Split(string(rawDataClean), ",")

	decoded := make([]uint32, len(str))
	log.Println("l", len(str))
	for i, s := range str {
		var d uint64
		d, err = strconv.ParseUint(s, 10, 32)
		if err != nil {
			return
		}
		gid := uint32(d)
		decoded[i] = gid
	}
	return decoded, err
}

func (m *Map) decodeLayerXML(l *Layer) (err error) {
	log.Println(len(l.Data.DataTiles))
	if len(l.Data.DataTiles) != m.Width*m.Height {
		return InvalidDecodedDataLen
	}

	l.DecodedTiles = make([]uint32, len(l.Data.DataTiles))

	for i, dataTile := range l.Data.DataTiles {
		l.DecodedTiles[i] = m.id(dataTile.GID)
	}

	return nil
}

func (m *Map) decodeLayerCSV(l *Layer) error {
	gids, err := l.Data.decodeCSV()
	if err != nil {
		return err
	}

	if len(gids) != m.Width*m.Height {
		return InvalidDecodedDataLen
	}

	l.DecodedTiles = make([]uint32, len(gids))

	for i, gid := range gids {
		l.DecodedTiles[i] = m.id(gid)
	}

	return nil
}

func (m *Map) id(gid uint32) uint32 {
	gidBare := gid &^ GID_FLIP

	if gidBare == 0 { // empty tile
		return NIL_TILE
	}

	for i := len(m.Tilesets) - 1; i >= 0; i-- {
		if m.Tilesets[i].FirstGID <= gidBare {
			return (gidBare - m.Tilesets[i].FirstGID) | (gid & GID_FLIP)
		}
	}

	panic("tmx: invalid GID")
}

func (m *Map) decodeLayerBase64(l *Layer) error {
	dataBytes, err := l.Data.decodeBase64()
	if err != nil {
		return err
	}

	if len(dataBytes) != m.Width*m.Height*4 {
		return InvalidDecodedDataLen
	}

	l.DecodedTiles = make([]uint32, m.Width*m.Height)

	j := 0
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			gid := uint32(dataBytes[j]) +
				uint32(dataBytes[j+1])<<8 +
				uint32(dataBytes[j+2])<<16 +
				uint32(dataBytes[j+3])<<24
			j += 4

			l.DecodedTiles[y*m.Width+x] = m.id(gid)
		}
	}

	return nil
}

func (m *Map) decodeLayer(l *Layer) error {
	switch l.Data.Encoding {
	case "csv":
		return m.decodeLayerCSV(l)
	case "base64":
		return m.decodeLayerBase64(l)
	case "": // XML "encoding"
		return m.decodeLayerXML(l)
	}
	return UnknownEncoding
}

func (m *Map) decodeLayers() error {
	for i := 0; i < len(m.Layers); i++ {
		if err := m.decodeLayer(&m.Layers[i]); err != nil {
			return err
		}
	}
	return nil
}

type Point struct {
	X int
	Y int
}

type DataTile struct {
	GID uint32 `xml:"gid,attr"`
}

func (p *Polygon) Decode() ([]Point, error) {
	return decodePoints(p.Points)
}
func (p *PolyLine) Decode() ([]Point, error) {
	return decodePoints(p.Points)
}

func decodePoints(s string) ([]Point, error) {
	panic("not implemented") // BUG(utkan); Handle points
	return []Point{}, nil
}

func NewMap(tmxpath string) (*Map, error) {
	f, err := os.Open(tmxpath)
	if err != nil {
		return nil, err
	}

	tmx, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	m := new(Map)
	err = xml.Unmarshal(tmx, m)
	if err != nil {
		return nil, err
	}

	return m, m.decodeLayers()
}
