// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 SaveEditors

package main

// GNF texture container (PS5-style, version 5) holding one linear BC7 image.
//
//  0x00  "GNF "
//  0x04  u32 contentsSize      header ends / pixel data starts at 8 + contentsSize
//  0x08  u8 version, u8 textureCount, ...
//  0x28  8 dwords: RDNA-style image descriptor (T#)
//          dword1 bits 20-28  format code   (181 = BC7 in this file)
//          dword1 bits 30-31 + dword2 bits 0-11   width-1
//          dword2 bits 14-27  height-1
//          dword3 bits 16-19  last mip level, bits 20-24 swizzle mode (0 = linear)
//  0x44  u32 data size
//  data  BC7 blocks, row-major, tightly packed

import (
	"encoding/binary"
	"fmt"
	"math"
)

const gnfFormatBC7 = 181

type GNF struct {
	Raw         []byte
	Version     int
	Width       int
	Height      int
	Format      int
	SwizzleMode int
	LastLevel   int
	DataStart   int
	DataSize    int
}

func ParseGNF(b []byte) (*GNF, error) {
	if len(b) < 0x100 || string(b[0:4]) != "GNF " {
		return nil, fmt.Errorf("not a GNF texture (missing 'GNF ' header)")
	}
	le := binary.LittleEndian
	g := &GNF{Raw: b}
	contents := int(le.Uint32(b[4:]))
	g.DataStart = 8 + contents
	g.Version = int(b[8])
	if b[9] != 1 {
		return nil, fmt.Errorf("GNF holds %d textures; only single-texture files are supported", b[9])
	}
	d1 := le.Uint32(b[0x2c:])
	d2 := le.Uint32(b[0x30:])
	d3 := le.Uint32(b[0x34:])
	g.Format = int((d1 >> 20) & 0x1ff)
	g.Width = int(((d2&0xfff)<<2)|(d1>>30)) + 1
	g.Height = int((d2>>14)&0x3fff) + 1
	g.LastLevel = int((d3 >> 16) & 0xf)
	g.SwizzleMode = int((d3 >> 20) & 0x1f)
	g.DataSize = int(le.Uint32(b[0x44:]))
	if g.DataStart < 0x48 || g.DataStart > len(b) {
		return nil, fmt.Errorf("GNF data offset 0x%x is outside the file", g.DataStart)
	}
	if g.DataStart+g.DataSize > len(b) {
		return nil, fmt.Errorf("GNF data (0x%x bytes at 0x%x) runs past end of file (%d bytes)", g.DataSize, g.DataStart, len(b))
	}
	if g.Format != gnfFormatBC7 {
		return nil, fmt.Errorf("texture format code %d is not supported (only BC7 = %d)", g.Format, gnfFormatBC7)
	}
	if g.SwizzleMode != 0 {
		return nil, fmt.Errorf("texture is swizzled (mode %d); only linear textures are supported", g.SwizzleMode)
	}
	want := ((g.Width + 3) / 4) * ((g.Height + 3) / 4) * 16
	if g.LastLevel != 0 || g.DataSize != want {
		return nil, fmt.Errorf("texture %dx%d has mipmaps or padding (data %d bytes, expected %d); not supported", g.Width, g.Height, g.DataSize, want)
	}
	return g, nil
}

func (g *GNF) RGBA() []byte {
	return DecodeBC7(g.Raw[g.DataStart:g.DataStart+g.DataSize], g.Width, g.Height)
}

// WithRGBA returns a new file: identical header, pixel data re-encoded from rgba.
func (g *GNF) WithRGBA(rgba []byte) ([]byte, error) {
	if len(rgba) != g.Width*g.Height*4 {
		return nil, fmt.Errorf("image must be %dx%d (got %d bytes, need %d)", g.Width, g.Height, len(rgba), g.Width*g.Height*4)
	}
	enc := EncodeBC7(rgba, g.Width, g.Height)
	if len(enc) != g.DataSize {
		return nil, fmt.Errorf("internal: encoded size %d != %d", len(enc), g.DataSize)
	}
	out := make([]byte, len(g.Raw))
	copy(out, g.Raw)
	copy(out[g.DataStart:], enc)
	return out, nil
}

// PSNR between two RGBA buffers (dB); 99 if identical.
func PSNR(a, b []byte) float64 {
	var se float64
	for i := range a {
		d := float64(a[i]) - float64(b[i])
		se += d * d
	}
	if se == 0 {
		return 99
	}
	mse := se / float64(len(a))
	return 10 * math.Log10(255*255/mse)
}
