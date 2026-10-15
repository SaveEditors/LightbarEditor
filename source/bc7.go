// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 SaveEditors

package main

// BC7 block decoder (all 8 modes) and a mode-6 encoder.
// Tables follow the BC7 spec (same values as Pillow's BcnDecode.c).

import "math"

type bc7ModeInfo struct{ ns, pb, rb, isb, cb, ab, epb, spb, ib, ib2 int }

var bc7Modes = [8]bc7ModeInfo{
	{3, 4, 0, 0, 4, 0, 1, 0, 3, 0},
	{2, 6, 0, 0, 6, 0, 0, 1, 3, 0},
	{3, 6, 0, 0, 5, 0, 0, 0, 2, 0},
	{2, 6, 0, 0, 7, 0, 1, 0, 2, 0},
	{1, 0, 2, 1, 5, 6, 0, 0, 2, 3},
	{1, 0, 2, 0, 7, 8, 0, 0, 2, 2},
	{1, 0, 0, 0, 7, 7, 1, 0, 4, 0},
	{2, 6, 0, 0, 5, 5, 1, 0, 2, 0},
}

var bc7SI2 = [64]uint16{
	0xcccc, 0x8888, 0xeeee, 0xecc8, 0xc880, 0xfeec, 0xfec8, 0xec80, 0xc800, 0xffec,
	0xfe80, 0xe800, 0xffe8, 0xff00, 0xfff0, 0xf000, 0xf710, 0x008e, 0x7100, 0x08ce,
	0x008c, 0x7310, 0x3100, 0x8cce, 0x088c, 0x3110, 0x6666, 0x366c, 0x17e8, 0x0ff0,
	0x718e, 0x399c, 0xaaaa, 0xf0f0, 0x5a5a, 0x33cc, 0x3c3c, 0x55aa, 0x9696, 0xa55a,
	0x73ce, 0x13c8, 0x324c, 0x3bdc, 0x6996, 0xc33c, 0x9966, 0x0660, 0x0272, 0x04e4,
	0x4e40, 0x2720, 0xc936, 0x936c, 0x39c6, 0x639c, 0x9336, 0x9cc6, 0x817e, 0xe718,
	0xccf0, 0x0fcc, 0x7744, 0xee22,
}

var bc7SI3 = [64]uint32{
	0xaa685050, 0x6a5a5040, 0x5a5a4200, 0x5450a0a8, 0xa5a50000, 0xa0a05050, 0x5555a0a0,
	0x5a5a5050, 0xaa550000, 0xaa555500, 0xaaaa5500, 0x90909090, 0x94949494, 0xa4a4a4a4,
	0xa9a59450, 0x2a0a4250, 0xa5945040, 0x0a425054, 0xa5a5a500, 0x55a0a0a0, 0xa8a85454,
	0x6a6a4040, 0xa4a45000, 0x1a1a0500, 0x0050a4a4, 0xaaa59090, 0x14696914, 0x69691400,
	0xa08585a0, 0xaa821414, 0x50a4a450, 0x6a5a0200, 0xa9a58000, 0x5090a0a8, 0xa8a09050,
	0x24242424, 0x00aa5500, 0x24924924, 0x24499224, 0x50a50a50, 0x500aa550, 0xaaaa4444,
	0x66660000, 0xa5a0a5a0, 0x50a050a0, 0x69286928, 0x44aaaa44, 0x66666600, 0xaa444444,
	0x54a854a8, 0x95809580, 0x96969600, 0xa85454a8, 0x80959580, 0xaa141414, 0x96960000,
	0xaaaa1414, 0xa05050a0, 0xa0a5a5a0, 0x96000000, 0x40804080, 0xa9a8a9a8, 0xaaaaaa44,
	0x2a4a5254,
}

var bc7AI0 = [64]int{15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15,
	15, 15, 15, 15, 2, 8, 2, 2, 8, 8, 15, 2, 8,
	2, 2, 8, 8, 2, 2, 15, 15, 6, 8, 2, 8, 15,
	15, 2, 8, 2, 2, 2, 15, 15, 6, 6, 2, 6, 8,
	15, 15, 2, 2, 15, 15, 15, 15, 15, 2, 2, 15}

var bc7AI1 = [64]int{3, 3, 15, 15, 8, 3, 15, 15, 8, 8, 6, 6, 6,
	5, 3, 3, 3, 3, 8, 15, 3, 3, 6, 10, 5, 8,
	8, 6, 8, 5, 15, 15, 8, 15, 3, 5, 6, 10, 8,
	15, 15, 3, 15, 5, 15, 15, 15, 15, 3, 15, 5, 5,
	5, 8, 5, 10, 5, 10, 8, 13, 15, 12, 3, 3}

var bc7AI2 = [64]int{15, 8, 8, 3, 15, 15, 3, 8, 15, 15, 15, 15, 15,
	15, 15, 8, 15, 8, 15, 3, 15, 8, 15, 8, 3, 15,
	6, 10, 15, 15, 10, 8, 15, 3, 15, 10, 10, 8, 9,
	10, 6, 15, 8, 15, 3, 6, 6, 8, 15, 3, 15, 15,
	15, 15, 15, 15, 15, 15, 15, 15, 3, 15, 15, 8}

var bc7W2 = []int{0, 21, 43, 64}
var bc7W3 = []int{0, 9, 18, 27, 37, 46, 55, 64}
var bc7W4 = []int{0, 4, 9, 13, 17, 21, 26, 30, 34, 38, 43, 47, 51, 55, 60, 64}

func bc7Weights(n int) []int {
	switch n {
	case 2:
		return bc7W2
	case 3:
		return bc7W3
	}
	return bc7W4
}

func getBits(src []byte, bit, count int) int {
	v := 0
	for i := 0; i < count; i++ {
		b := bit + i
		if (src[b>>3]>>(b&7))&1 != 0 {
			v |= 1 << i
		}
	}
	return v
}

func bc7Subset(ns, partition, n int) int {
	switch ns {
	case 2:
		return int(1 & (bc7SI2[partition] >> n))
	case 3:
		return int(3 & (bc7SI3[partition] >> (2 * n)))
	}
	return 0
}

// decodeBC7Block writes 16 RGBA pixels (64 bytes, row-major 4x4) into out.
func decodeBC7Block(src []byte, out []byte) {
	mode := -1
	for i := 0; i < 8; i++ {
		if (src[0]>>i)&1 != 0 {
			mode = i
			break
		}
	}
	if mode < 0 { // reserved: spec says output transparent black; Pillow uses opaque black
		for i := 0; i < 16; i++ {
			out[i*4], out[i*4+1], out[i*4+2], out[i*4+3] = 0, 0, 0, 255
		}
		return
	}
	info := bc7Modes[mode]
	bit := mode + 1
	load := func(n int) int { v := getBits(src, bit, n); bit += n; return v }
	partition := load(info.pb)
	rotation := load(info.rb)
	indexSel := load(info.isb)
	numep := info.ns * 2
	var ep [6][4]int
	cb, ab := info.cb, info.ab
	for c := 0; c < 3; c++ {
		for i := 0; i < numep; i++ {
			ep[i][c] = load(cb)
		}
	}
	for i := 0; i < numep; i++ {
		if ab > 0 {
			ep[i][3] = load(ab)
		} else {
			ep[i][3] = 255
		}
	}
	if info.epb != 0 {
		cb++
		if ab > 0 {
			ab++
		}
		for i := 0; i < numep; i++ {
			p := load(1)
			for c := 0; c < 3; c++ {
				ep[i][c] = ep[i][c]<<1 | p
			}
			if ab > 0 {
				ep[i][3] = ep[i][3]<<1 | p
			}
		}
	}
	if info.spb != 0 {
		cb++
		if ab > 0 {
			ab++
		}
		for i := 0; i < numep; i += 2 {
			p := load(1)
			for j := 0; j < 2; j++ {
				for c := 0; c < 3; c++ {
					ep[i+j][c] = ep[i+j][c]<<1 | p
				}
				if ab > 0 {
					ep[i+j][3] = ep[i+j][3]<<1 | p
				}
			}
		}
	}
	expand := func(v, bits int) int { v = (v << (8 - bits)) & 0xff; return v | (v >> bits) }
	for i := 0; i < numep; i++ {
		for c := 0; c < 3; c++ {
			ep[i][c] = expand(ep[i][c], cb)
		}
		if ab > 0 {
			ep[i][3] = expand(ep[i][3], ab)
		}
	}
	cw := bc7Weights(info.ib)
	aw := cw
	if ab > 0 && info.ib2 > 0 {
		aw = bc7Weights(info.ib2)
	}
	cibit := bit
	aibit := cibit + 16*info.ib - info.ns
	for i := 0; i < 16; i++ {
		s := bc7Subset(info.ns, partition, i) << 1
		ib := info.ib
		if i == 0 {
			ib--
		} else if info.ns == 2 && i == bc7AI0[partition] {
			ib--
		} else if info.ns == 3 && (i == bc7AI1[partition] || i == bc7AI2[partition]) {
			ib--
		}
		i0 := getBits(src, cibit, ib)
		cibit += ib
		var s0, s1 int
		if ab > 0 && info.ib2 > 0 {
			ib2 := info.ib2
			if i == 0 {
				ib2--
			}
			i1 := getBits(src, aibit, ib2)
			aibit += ib2
			if indexSel != 0 {
				s0, s1 = aw[i1], cw[i0]
			} else {
				s0, s1 = cw[i0], aw[i1]
			}
		} else {
			s0, s1 = cw[i0], cw[i0]
		}
		e0, e1 := ep[s], ep[s+1]
		px := out[i*4 : i*4+4]
		for c := 0; c < 3; c++ {
			px[c] = byte(((64-s0)*e0[c] + s0*e1[c] + 32) >> 6)
		}
		px[3] = byte(((64-s1)*e0[3] + s1*e1[3] + 32) >> 6)
		switch rotation {
		case 1:
			px[0], px[3] = px[3], px[0]
		case 2:
			px[1], px[3] = px[3], px[1]
		case 3:
			px[2], px[3] = px[3], px[2]
		}
	}
}

// DecodeBC7 decodes a tightly packed linear BC7 image to RGBA8.
func DecodeBC7(data []byte, w, h int) []byte {
	bw, bh := (w+3)/4, (h+3)/4
	out := make([]byte, w*h*4)
	var blk [64]byte
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			off := (by*bw + bx) * 16
			decodeBC7Block(data[off:off+16], blk[:])
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					px, py := bx*4+x, by*4+y
					if px < w && py < h {
						copy(out[(py*w+px)*4:(py*w+px)*4+4], blk[(y*4+x)*4:(y*4+x)*4+4])
					}
				}
			}
		}
	}
	return out
}

// ---------------- mode 6 encoder ----------------

type bitWriter struct {
	buf [16]byte
	pos int
}

func (b *bitWriter) put(v, n int) {
	for i := 0; i < n; i++ {
		if (v>>i)&1 != 0 {
			b.buf[b.pos>>3] |= 1 << (b.pos & 7)
		}
		b.pos++
	}
}

type m6cand struct {
	q   [2][4]int // 7-bit endpoint values
	p   [2]int    // p-bits
	idx [16]int
	err float64
}

func m6Endpoint(q [4]int, p int) [4]int {
	var e [4]int
	for c := 0; c < 4; c++ {
		e[c] = q[c]<<1 | p
	}
	return e
}

func m6Eval(px *[16][4]float64, q [2][4]int, p [2]int) m6cand {
	e0, e1 := m6Endpoint(q[0], p[0]), m6Endpoint(q[1], p[1])
	var pal [16][4]float64
	for k := 0; k < 16; k++ {
		w := bc7W4[k]
		for c := 0; c < 4; c++ {
			pal[k][c] = float64(((64-w)*e0[c] + w*e1[c] + 32) >> 6)
		}
	}
	r := m6cand{q: q, p: p}
	for i := 0; i < 16; i++ {
		best, bi := math.MaxFloat64, 0
		for k := 0; k < 16; k++ {
			d := 0.0
			for c := 0; c < 4; c++ {
				t := px[i][c] - pal[k][c]
				d += t * t
			}
			if d < best {
				best, bi = d, k
			}
		}
		r.idx[i] = bi
		r.err += best
	}
	return r
}

func quant7(v float64, p int) int {
	q := int(math.Floor((v-float64(p))/2 + 0.5))
	if q < 0 {
		q = 0
	}
	if q > 127 {
		q = 127
	}
	return q
}

func m6Best(px *[16][4]float64, e0, e1 [4]float64) m6cand {
	best := m6cand{err: math.MaxFloat64}
	for p0 := 0; p0 < 2; p0++ {
		for p1 := 0; p1 < 2; p1++ {
			var q [2][4]int
			for c := 0; c < 4; c++ {
				q[0][c] = quant7(e0[c], p0)
				q[1][c] = quant7(e1[c], p1)
			}
			r := m6Eval(px, q, [2]int{p0, p1})
			if r.err < best.err {
				best = r
			}
		}
	}
	return best
}

// encodeBC7Mode6Block encodes 16 RGBA pixels (64 bytes) into a 16-byte block.
func encodeBC7Mode6Block(rgba []byte) [16]byte {
	var px [16][4]float64
	var mean [4]float64
	for i := 0; i < 16; i++ {
		for c := 0; c < 4; c++ {
			px[i][c] = float64(rgba[i*4+c])
			mean[c] += px[i][c] / 16
		}
	}
	// principal axis via power iteration
	var cov [4][4]float64
	for i := 0; i < 16; i++ {
		for a := 0; a < 4; a++ {
			for b := 0; b < 4; b++ {
				cov[a][b] += (px[i][a] - mean[a]) * (px[i][b] - mean[b])
			}
		}
	}
	axis := [4]float64{1, 1, 1, 0.5}
	for it := 0; it < 12; it++ {
		var n [4]float64
		for a := 0; a < 4; a++ {
			for b := 0; b < 4; b++ {
				n[a] += cov[a][b] * axis[b]
			}
		}
		l := math.Sqrt(n[0]*n[0] + n[1]*n[1] + n[2]*n[2] + n[3]*n[3])
		if l < 1e-9 {
			break
		}
		for a := 0; a < 4; a++ {
			axis[a] = n[a] / l
		}
	}
	tmin, tmax := math.MaxFloat64, -math.MaxFloat64
	for i := 0; i < 16; i++ {
		t := 0.0
		for c := 0; c < 4; c++ {
			t += (px[i][c] - mean[c]) * axis[c]
		}
		tmin = math.Min(tmin, t)
		tmax = math.Max(tmax, t)
	}
	var e0, e1 [4]float64
	for c := 0; c < 4; c++ {
		e0[c] = mean[c] + tmin*axis[c]
		e1[c] = mean[c] + tmax*axis[c]
	}
	best := m6Best(&px, e0, e1)
	// least-squares refinement
	for iter := 0; iter < 4 && best.err > 0; iter++ {
		var aa, ab, bb float64
		var ax, bx [4]float64
		for i := 0; i < 16; i++ {
			w := float64(bc7W4[best.idx[i]]) / 64
			a, b := 1-w, w
			aa += a * a
			ab += a * b
			bb += b * b
			for c := 0; c < 4; c++ {
				ax[c] += a * px[i][c]
				bx[c] += b * px[i][c]
			}
		}
		det := aa*bb - ab*ab
		if math.Abs(det) < 1e-9 {
			break
		}
		for c := 0; c < 4; c++ {
			e0[c] = (bb*ax[c] - ab*bx[c]) / det
			e1[c] = (aa*bx[c] - ab*ax[c]) / det
		}
		r := m6Best(&px, e0, e1)
		if r.err >= best.err {
			break
		}
		best = r
	}
	// local search: nudge each endpoint channel by +-1
	for pass := 0; pass < 2 && best.err > 0; pass++ {
		improved := false
		for e := 0; e < 2; e++ {
			for c := 0; c < 4; c++ {
				for _, d := range []int{-1, 1} {
					q := best.q
					q[e][c] += d
					if q[e][c] < 0 || q[e][c] > 127 {
						continue
					}
					r := m6Eval(&px, q, best.p)
					if r.err < best.err {
						best, improved = r, true
					}
				}
			}
		}
		if !improved {
			break
		}
	}
	// anchor: index 0 must have its MSB clear
	if best.idx[0] >= 8 {
		best.q[0], best.q[1] = best.q[1], best.q[0]
		best.p[0], best.p[1] = best.p[1], best.p[0]
		for i := range best.idx {
			best.idx[i] = 15 - best.idx[i]
		}
	}
	var bw bitWriter
	bw.put(1<<6, 7)
	for c := 0; c < 4; c++ {
		bw.put(best.q[0][c], 7)
		bw.put(best.q[1][c], 7)
	}
	bw.put(best.p[0], 1)
	bw.put(best.p[1], 1)
	for i := 0; i < 16; i++ {
		if i == 0 {
			bw.put(best.idx[i], 3)
		} else {
			bw.put(best.idx[i], 4)
		}
	}
	return bw.buf
}

// EncodeBC7 encodes RGBA8 (w*h*4) into tightly packed BC7 blocks.
func EncodeBC7(rgba []byte, w, h int) []byte {
	bw, bh := (w+3)/4, (h+3)/4
	out := make([]byte, 0, bw*bh*16)
	blk := make([]byte, 64)
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					px, py := bx*4+x, by*4+y
					if px >= w {
						px = w - 1
					}
					if py >= h {
						py = h - 1
					}
					copy(blk[(y*4+x)*4:(y*4+x)*4+4], rgba[(py*w+px)*4:(py*w+px)*4+4])
				}
			}
			b := encodeBC7Mode6Block(blk)
			out = append(out, b[:]...)
		}
	}
	return out
}
