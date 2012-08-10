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
	"encoding/csv"
	"encoding/xml"
	"errors"
	"io"
	"io/ioutil"
	"os"
)

const (
	GID_HORIZONTAL_FLIP = 0x80000000
	GID_VERTICAL_FLIP   = 0x40000000
	GID_DIAGONAL_FLIP   = 0x20000000
	GID_FLIP            = GID_HORIZONTAL_FLIP | GID_VERTICAL_FLIP | GID_DIAGONAL_FLIP
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
	Encoding    string `xml:"encoding,attr"`
	Compression string `xml:"compression,attr"`
	RawData     []byte `xml:",innerxml"`
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

type csvReader struct {
	r *csv.Reader
}

func (r *csvReader) Read([]byte) (int, error) {
	panic("not implemented") // BUG(utkan); Handle CSV
}

func newCSVReader(r io.Reader) *csvReader {
	return &csvReader{csv.NewReader(r)}
}

func (d *Data) decode() (data []byte, err error) {
	rawData := bytes.TrimSpace(d.RawData)
	r := bytes.NewReader(rawData)

	var encr io.Reader
	switch d.Encoding {
	case "base64":
		encr = base64.NewDecoder(base64.StdEncoding, r)
	case "csv":
		encr = newCSVReader(r)
	default:
		err = UnknownEncoding
		return
	}

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
		panic("unknown")
		return
	}

	return ioutil.ReadAll(comr)
}

func (m *Map) decodeLayer(l *Layer) error {
	dataBytes, err := l.Data.decode()
	if err != nil {
		return err
	}

	if len(dataBytes) != m.Width*m.Height*4 {
		return InvalidDecodedDataLen
	}

	l.DecodedTiles = make([]uint32, m.Width*m.Height)

	id := func(gid uint32) (uint32, error) {
		gidBare := gid &^ GID_FLIP
		
		if gidBare == 0 { // empty tile
			return 0, nil
		}

		for i := len(m.Tilesets) - 1; i >= 0; i-- {
			if m.Tilesets[i].FirstGID <= gidBare {
				return (gidBare - m.Tilesets[i].FirstGID) | (gid & GID_FLIP), nil
			}
		}

		return 0, InvalidGID
	}

	j := 0
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			gid := uint32(dataBytes[j]) +
				uint32(dataBytes[j+1])<<8 +
				uint32(dataBytes[j+2])<<16 +
				uint32(dataBytes[j+3])<<24
			j += 4

			l.DecodedTiles[y*m.Width+x], err = id(gid)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Map) decodeLayers() error {
	for i:=0; i<len(m.Layers); i++ {
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
