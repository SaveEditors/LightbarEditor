// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 SaveEditors

package main

// .fbxd ribbon mesh (see README for the full layout).

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

const vStride = 48

type Mesh struct {
	Raw            []byte
	Count          uint64
	VC, IC         int
	VStart, IStart int
	Pos            []float32 // 3 per vertex
	UV             []float32 // 2 per vertex
	Col            []float32 // 4 per vertex
	Idx            []uint16
}

func f32(b []byte) float32       { return math.Float32frombits(binary.LittleEndian.Uint32(b)) }
func putf32(b []byte, v float32) { binary.LittleEndian.PutUint32(b, math.Float32bits(v)) }

func ParseFBXD(b []byte) (*Mesh, error) {
	le := binary.LittleEndian
	if len(b) < 0x30 {
		return nil, fmt.Errorf("file is too small (%d bytes) to be an .fbxd mesh", len(b))
	}
	m := &Mesh{Raw: b}
	m.Count = le.Uint64(b[0:])
	if m.Count != 1 {
		return nil, fmt.Errorf("submesh count is %d; this editor only understands single-mesh .fbxd files", m.Count)
	}
	if le.Uint64(b[8:]) != 8 || le.Uint64(b[16:]) != 8 {
		return nil, fmt.Errorf("unexpected header layout (not a lightbar-style .fbxd)")
	}
	m.VC = int(le.Uint16(b[0x18:]))
	m.IC = int(le.Uint16(b[0x1a:]))
	vo, io := le.Uint64(b[0x20:]), le.Uint64(b[0x28:])
	if vo > uint64(len(b)) || io > uint64(len(b)) {
		return nil, fmt.Errorf("buffer offsets point outside the file")
	}
	m.VStart, m.IStart = 0x20+int(vo), 0x28+int(io)
	if m.VStart < 0x30 || m.VStart+m.VC*vStride > len(b) {
		return nil, fmt.Errorf("vertex buffer (%d vertices) does not fit in the file", m.VC)
	}
	if m.IStart < m.VStart+m.VC*vStride || m.IStart+m.IC*2 > len(b) {
		return nil, fmt.Errorf("index buffer (%d indices) does not fit in the file", m.IC)
	}
	if m.IC%3 != 0 {
		return nil, fmt.Errorf("index count %d is not a multiple of 3", m.IC)
	}
	m.Pos = make([]float32, m.VC*3)
	m.UV = make([]float32, m.VC*2)
	m.Col = make([]float32, m.VC*4)
	for i := 0; i < m.VC; i++ {
		o := m.VStart + i*vStride
		for c := 0; c < 3; c++ {
			m.Pos[i*3+c] = f32(b[o+c*4:])
		}
		m.UV[i*2] = f32(b[o+16:])
		m.UV[i*2+1] = f32(b[o+20:])
		for c := 0; c < 4; c++ {
			m.Col[i*4+c] = f32(b[o+32+c*4:])
		}
	}
	for _, v := range append(append(append([]float32{}, m.Pos...), m.UV...), m.Col...) {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, fmt.Errorf("vertex data contains NaN/Inf – file is corrupt or not an .fbxd mesh")
		}
	}
	m.Idx = make([]uint16, m.IC)
	for i := 0; i < m.IC; i++ {
		m.Idx[i] = le.Uint16(b[m.IStart+i*2:])
		if int(m.Idx[i]) >= m.VC {
			return nil, fmt.Errorf("triangle index %d refers to vertex %d but there are only %d", i, m.Idx[i], m.VC)
		}
	}
	return m, nil
}

// Strands returns connected vertex groups, each sorted by distance (uv.v).
func (m *Mesh) Strands() [][]int {
	parent := make([]int, m.VC)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	for t := 0; t+2 < m.IC; t += 3 {
		a := find(int(m.Idx[t]))
		parent[find(int(m.Idx[t+1]))] = a
		parent[find(int(m.Idx[t+2]))] = a
	}
	groups := map[int][]int{}
	var roots []int
	for i := 0; i < m.VC; i++ {
		r := find(i)
		if _, ok := groups[r]; !ok {
			roots = append(roots, r)
		}
		groups[r] = append(groups[r], i)
	}
	var out [][]int
	for _, r := range roots {
		g := groups[r]
		sort.SliceStable(g, func(a, b int) bool { return m.UV[g[a]*2+1] < m.UV[g[b]*2+1] })
		out = append(out, g)
	}
	sort.SliceStable(out, func(a, b int) bool { return minInt(out[a]) < minInt(out[b]) })
	return out
}

func minInt(s []int) int {
	m := s[0]
	for _, v := range s {
		if v < m {
			m = v
		}
	}
	return m
}

// Patched returns a copy of the original file with new positions/colours written in place.
func (m *Mesh) Patched(pos, col []float32) ([]byte, error) {
	if len(pos) != m.VC*3 || len(col) != m.VC*4 {
		return nil, fmt.Errorf("edit data has wrong size (pos %d, col %d; need %d, %d)", len(pos), len(col), m.VC*3, m.VC*4)
	}
	for _, v := range append(append([]float32{}, pos...), col...) {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, fmt.Errorf("edit contains NaN/Inf values")
		}
	}
	out := make([]byte, len(m.Raw))
	copy(out, m.Raw)
	for i := 0; i < m.VC; i++ {
		o := m.VStart + i*vStride
		for c := 0; c < 3; c++ {
			putf32(out[o+c*4:], pos[i*3+c])
		}
		for c := 0; c < 4; c++ {
			putf32(out[o+32+c*4:], col[i*4+c])
		}
	}
	// verify
	m2, err := ParseFBXD(out)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %v", err)
	}
	if m2.VC != m.VC || m2.IC != m.IC {
		return nil, fmt.Errorf("verification failed: counts changed")
	}
	return out, nil
}

// ---------- colour operation (shared by CLI and editor semantics) ----------

type ColorOp struct {
	Strand     int // -1 = all
	A, B       [3]float64
	Gradient   bool
	Solid      bool // ignore original fade
	Brightness float64
}

func (m *Mesh) ApplyColor(op ColorOp) error {
	strands := m.Strands()
	for si, s := range strands {
		if op.Strand >= 0 && op.Strand != si {
			continue
		}
		vmin, vmax := math.Inf(1), math.Inf(-1)
		for _, i := range s {
			v := float64(m.UV[i*2+1])
			vmin, vmax = math.Min(vmin, v), math.Max(vmax, v)
		}
		rng := vmax - vmin
		if rng <= 0 {
			rng = 1
		}
		for _, i := range s {
			t := (float64(m.UV[i*2+1]) - vmin) / rng
			k := op.Brightness
			if !op.Solid {
				k *= math.Max(float64(m.Col[i*4]), math.Max(float64(m.Col[i*4+1]), float64(m.Col[i*4+2])))
			}
			for c := 0; c < 3; c++ {
				col := op.A[c]
				if op.Gradient {
					col = op.A[c] + (op.B[c]-op.A[c])*t
				}
				m.Col[i*4+c] = float32(col * k)
			}
		}
	}
	if op.Strand >= len(strands) {
		return fmt.Errorf("strip %d does not exist (file has %d)", op.Strand, len(strands))
	}
	return nil
}

func ParseColor(s string) ([3]float64, error) {
	s = strings.TrimSpace(strings.TrimPrefix(s, "#"))
	if strings.Contains(s, ",") {
		p := strings.Split(s, ",")
		if len(p) != 3 {
			return [3]float64{}, fmt.Errorf("colour '%s': use #rrggbb or r,g,b (0-1)", s)
		}
		var c [3]float64
		for i := range p {
			v, err := strconv.ParseFloat(strings.TrimSpace(p[i]), 64)
			if err != nil {
				return c, fmt.Errorf("colour '%s': %v", s, err)
			}
			c[i] = v
		}
		return c, nil
	}
	if len(s) != 6 {
		return [3]float64{}, fmt.Errorf("colour '%s': use #rrggbb or r,g,b (0-1)", s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return [3]float64{}, fmt.Errorf("colour '%s': %v", s, err)
	}
	return [3]float64{float64(b[0]) / 255, float64(b[1]) / 255, float64(b[2]) / 255}, nil
}

// ---------- JSON (same format as fbxd_tool.py) ----------

type jsonVertex struct {
	Pos   [3]float32 `json:"pos"`
	W     float32    `json:"w"`
	UV    [2]float32 `json:"uv"`
	Color [4]float32 `json:"color"`
	Pad   [2]uint32  `json:"pad"`
}

type jsonMesh struct {
	Header struct {
		SubmeshCount uint64 `json:"submesh_count"`
		Off1         uint64 `json:"off1"`
		Off2         uint64 `json:"off2"`
		Pad          uint32 `json:"pad"`
	} `json:"header"`
	Vertices   []jsonVertex `json:"vertices"`
	Indices    []uint16     `json:"indices"`
	TrailerHex string       `json:"trailer_hex"`
}

func (m *Mesh) ToJSON() []byte {
	le := binary.LittleEndian
	var j jsonMesh
	j.Header.SubmeshCount = m.Count
	j.Header.Off1, j.Header.Off2 = le.Uint64(m.Raw[8:]), le.Uint64(m.Raw[16:])
	j.Header.Pad = le.Uint32(m.Raw[0x1c:])
	for i := 0; i < m.VC; i++ {
		o := m.VStart + i*vStride
		var v jsonVertex
		copy(v.Pos[:], m.Pos[i*3:i*3+3])
		v.W = f32(m.Raw[o+12:])
		copy(v.UV[:], m.UV[i*2:i*2+2])
		copy(v.Color[:], m.Col[i*4:i*4+4])
		v.Pad = [2]uint32{le.Uint32(m.Raw[o+24:]), le.Uint32(m.Raw[o+28:])}
		j.Vertices = append(j.Vertices, v)
	}
	j.Indices = m.Idx
	j.TrailerHex = hex.EncodeToString(m.Raw[m.IStart+m.IC*2:])
	// one vertex per line
	var sb strings.Builder
	h, _ := json.Marshal(j.Header)
	sb.WriteString("{\n  \"header\": " + string(h) + ",\n  \"vertices\": [\n")
	for i, v := range j.Vertices {
		b, _ := json.Marshal(v)
		sb.WriteString("    " + string(b))
		if i < len(j.Vertices)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("  ],\n  \"indices\": [\n")
	for t := 0; t < len(j.Indices); t += 3 {
		sb.WriteString(fmt.Sprintf("    %d,%d,%d", j.Indices[t], j.Indices[t+1], j.Indices[t+2]))
		if t+3 < len(j.Indices) {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("  ],\n  \"trailer_hex\": \"" + j.TrailerHex + "\"\n}\n")
	return []byte(sb.String())
}

func FBXDFromJSON(data []byte) ([]byte, error) {
	var j jsonMesh
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("JSON error: %v", err)
	}
	if len(j.Vertices) == 0 || len(j.Vertices) > 0xffff || len(j.Indices) > 0xffff {
		return nil, fmt.Errorf("vertex/index count out of range (vertices %d, indices %d; max 65535)", len(j.Vertices), len(j.Indices))
	}
	if len(j.Indices)%3 != 0 {
		return nil, fmt.Errorf("index count %d is not a multiple of 3", len(j.Indices))
	}
	for _, ix := range j.Indices {
		if int(ix) >= len(j.Vertices) {
			return nil, fmt.Errorf("index %d refers to a missing vertex", ix)
		}
	}
	tr, err := hex.DecodeString(j.TrailerHex)
	if err != nil {
		return nil, fmt.Errorf("trailer_hex: %v", err)
	}
	le := binary.LittleEndian
	vstart := 0x30
	istart := vstart + len(j.Vertices)*vStride
	out := make([]byte, istart+len(j.Indices)*2+len(tr))
	le.PutUint64(out[0:], j.Header.SubmeshCount)
	le.PutUint64(out[8:], j.Header.Off1)
	le.PutUint64(out[16:], j.Header.Off2)
	le.PutUint16(out[0x18:], uint16(len(j.Vertices)))
	le.PutUint16(out[0x1a:], uint16(len(j.Indices)))
	le.PutUint32(out[0x1c:], j.Header.Pad)
	le.PutUint64(out[0x20:], uint64(vstart-0x20))
	le.PutUint64(out[0x28:], uint64(istart-0x28))
	for i, v := range j.Vertices {
		o := vstart + i*vStride
		for c := 0; c < 3; c++ {
			putf32(out[o+c*4:], v.Pos[c])
		}
		putf32(out[o+12:], v.W)
		putf32(out[o+16:], v.UV[0])
		putf32(out[o+20:], v.UV[1])
		le.PutUint32(out[o+24:], v.Pad[0])
		le.PutUint32(out[o+28:], v.Pad[1])
		for c := 0; c < 4; c++ {
			putf32(out[o+32+c*4:], v.Color[c])
		}
	}
	for i, ix := range j.Indices {
		le.PutUint16(out[istart+i*2:], ix)
	}
	copy(out[istart+len(j.Indices)*2:], tr)
	if _, err := ParseFBXD(out); err != nil {
		return nil, fmt.Errorf("rebuilt file failed verification: %v", err)
	}
	return out, nil
}

func (m *Mesh) WriteOBJ(path string) error {
	var sb strings.Builder
	sb.WriteString("# decoded .fbxd - vertex colours after xyz\n")
	for i := 0; i < m.VC; i++ {
		sb.WriteString(fmt.Sprintf("v %g %g %g %g %g %g\n", m.Pos[i*3], m.Pos[i*3+1], m.Pos[i*3+2], m.Col[i*4], m.Col[i*4+1], m.Col[i*4+2]))
	}
	for i := 0; i < m.VC; i++ {
		sb.WriteString(fmt.Sprintf("vt %g %g\n", m.UV[i*2], m.UV[i*2+1]))
	}
	for si, s := range m.Strands() {
		sb.WriteString(fmt.Sprintf("o strip_%d\n", si))
		in := map[int]bool{}
		for _, i := range s {
			in[i] = true
		}
		for t := 0; t < m.IC; t += 3 {
			a, b, c := int(m.Idx[t])+1, int(m.Idx[t+1])+1, int(m.Idx[t+2])+1
			if in[a-1] {
				sb.WriteString(fmt.Sprintf("f %d/%d %d/%d %d/%d\n", a, a, b, b, c, c))
			}
		}
	}
	return os.WriteFile(path, []byte(sb.String()), 0644)
}
