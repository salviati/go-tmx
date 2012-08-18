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
	"github.com/salviati/go-tmx/tmx"
	"strconv"
)

const (
	GBAHFlip = 1 << 10
	GBAVFlip = 1 << 11
)

type GBA struct {
	isAffineCache map[*tmx.Layer]bool // FIXME(utkan): One may get a pointer collision. Fail-safe practice would be to use a new GBA instance for each map.
	nilTileCache  map[*tmx.Layer]uint16
}

func (g *GBA) MaxTiles(m *tmx.Map, l *tmx.Layer) int {
	// Save one for nil-tile. FIXME(utkan): Not everone will need an extra tile.
	if g.isAffine(l) {
		return 255
	}
	return 511
}

func (g *GBA) isAffine(l *tmx.Layer) bool {
	if g.isAffineCache == nil {
		g.isAffineCache = make(map[*tmx.Layer]bool)
	}

	affine, ok := g.isAffineCache[l]
	if !ok {
		affineString, _ := GetProperty(&l.Properties, "Affine")
		affine = affineString == "true"
		g.isAffineCache[l] = affine
	}

	return affine
}

func (g *GBA) nilTile(m *tmx.Map, l *tmx.Layer) interface{} {
	if g.nilTileCache == nil {
		g.nilTileCache = make(map[*tmx.Layer]uint16)
	}

	nilTile, ok := g.nilTileCache[l]
	if !ok {
		nilTile := uint16(len(l.Tileset.Tiles))
		nilTileString, _ := GetProperty(&l.Properties, "NilTile")
		nilTileNew, err := strconv.ParseUint(nilTileString, 10, 16)
		if err == nil {
			nilTile = uint16(nilTileNew)
		}
		g.nilTileCache[l] = nilTile
	}

	if g.isAffine(l) {
		return uint8(nilTile)
	}
	return nilTile
}

func (g *GBA) ScreenblockEntry(m *tmx.Map, l *tmx.Layer, tile *tmx.DecodedTile) (interface{}, error) {
	affine := g.isAffine(l)

	rval := func(v uint16) interface{} {
		if affine {
			return uint8(v)
		}
		return uint16(v)
	}

	if tile.IsNil() {
		return g.nilTile(m, l), nil
	}

	tid := uint16(tile.ID)
	if affine {
		return rval(tid), nil
	}

	if tile.HorizontalFlip {
		tid |= GBAHFlip
	}
	if tile.VerticalFlip {
		tid |= GBAVFlip
	}
	// TODO(utkan): palette bank
	return rval(tid), nil
}

func (g *GBA) ByteOrder() binary.ByteOrder {
	return binary.LittleEndian
}
