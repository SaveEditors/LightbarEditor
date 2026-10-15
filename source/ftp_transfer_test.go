// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 SaveEditors

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Synthetic containers keep console assets out of the source distribution.
func testPair() map[string][]byte {
	m := make([]byte, 0x30+3*48+6)
	le := binary.LittleEndian
	le.PutUint64(m, 1)
	le.PutUint64(m[8:], 8)
	le.PutUint64(m[16:], 8)
	le.PutUint16(m[0x18:], 3)
	le.PutUint16(m[0x1a:], 3)
	le.PutUint64(m[0x20:], 0x10)
	le.PutUint64(m[0x28:], uint64(0x30+3*48-0x28))
	le.PutUint16(m[len(m)-4:], 1)
	le.PutUint16(m[len(m)-2:], 2)
	g := make([]byte, 272)
	copy(g, "GNF ")
	le.PutUint32(g[4:], 248)
	g[8] = 5
	g[9] = 1
	le.PutUint32(g[0x2c:], 181<<20|3<<30)
	le.PutUint32(g[0x30:], 3<<14)
	le.PutUint32(g[0x44:], 16)
	return map[string][]byte{transferFiles[0]: m, transferFiles[1]: g}
}

type fakeFTP struct {
	data      map[string][]byte
	writes    int
	failAt    int
	corruptAt int
	failAll   bool
}

func (f *fakeFTP) Read(p string) ([]byte, error) {
	b, ok := f.data[filepath.Base(p)]
	if !ok {
		return nil, fmt.Errorf("missing")
	}
	return bytes.Clone(b), nil
}
func (f *fakeFTP) Write(p string, b []byte) error {
	f.writes++
	name := filepath.Base(p)
	f.data[name] = bytes.Clone(b)
	if f.failAll || f.writes == f.failAt {
		f.data[name] = []byte("partial")
		return fmt.Errorf("connection lost")
	}
	if f.writes == f.corruptAt {
		f.data[name][len(b)-1] ^= 1
	}
	return nil
}
func (f *fakeFTP) Close() {}
func setupTransfer(t *testing.T) (*transferService, *fakeFTP, ftpConfig, string, string) {
	t.Helper()
	root := t.TempDir()
	s := newTransferService(root)
	f := &fakeFTP{data: testPair()}
	s.dial = func(ftpConfig) (assetFTP, error) { return f, nil }
	cfg := ftpConfig{Host: "console.example", Port: 1337, Directory: "/system_ex/vsh_asset", Password: "secret-do-not-save"}
	pair := testPair()
	pair[transferFiles[0]][0x30+32] = 1
	pair[transferFiles[1]][271] = 8
	paths := []string{}
	for _, name := range transferFiles {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, pair[name], 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return s, f, cfg, paths[0], paths[1]
}
func prepareTest(t *testing.T, s *transferService, c ftpConfig, m, g string) map[string]any {
	t.Helper()
	r, e := s.Prepare(c, m, g)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestTransferVerifiedAndSingleUse(t *testing.T) {
	s, f, c, m, g := setupTransfer(t)
	original := bytes.Clone(f.data[transferFiles[0]])
	r := prepareTest(t, s, c, m, g)
	if f.writes != 0 {
		t.Fatal("preparation wrote to console")
	}
	backup := r["backup"].(string)
	b, e := os.ReadFile(filepath.Join(backup, transferFiles[0]))
	if e != nil || !bytes.Equal(b, original) {
		t.Fatal("bad backup")
	}
	manifest, _ := os.ReadFile(filepath.Join(backup, "manifest.json"))
	if strings.Contains(string(manifest), c.Password) {
		t.Fatal("password persisted")
	}
	if _, e = s.Upload(r["plan"].(string)); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{m, g} {
		want, _ := os.ReadFile(p)
		if !bytes.Equal(want, f.data[filepath.Base(p)]) {
			t.Fatal("wrong uploaded bytes")
		}
	}
	if _, e = s.Upload(r["plan"].(string)); e == nil {
		t.Fatal("plan reused")
	}
}
func TestTransferRollback(t *testing.T) {
	for _, mode := range []string{"partial-second", "readback", "restoration-failed"} {
		t.Run(mode, func(t *testing.T) {
			s, f, c, m, g := setupTransfer(t)
			original := testPair()
			r := prepareTest(t, s, c, m, g)
			if mode == "readback" {
				f.corruptAt = 1
			} else {
				f.failAt = 2
			}
			if mode == "restoration-failed" {
				f.failAll = true
			}
			_, e := s.Upload(r["plan"].(string))
			if e == nil {
				t.Fatal("failure not reported")
			}
			if mode == "restoration-failed" {
				if !strings.Contains(e.Error(), "restoration incomplete") {
					t.Fatal(e)
				}
				return
			}
			if !strings.Contains(e.Error(), "restored and verified") {
				t.Fatal(e)
			}
			for _, n := range transferFiles {
				if !bytes.Equal(original[n], f.data[n]) {
					t.Fatal("rollback mismatch", n)
				}
			}
		})
	}
}
func TestTransferRejectsStaleReview(t *testing.T) {
	for _, mode := range []string{"local", "remote", "expired"} {
		t.Run(mode, func(t *testing.T) {
			s, f, c, m, g := setupTransfer(t)
			r := prepareTest(t, s, c, m, g)
			id := r["plan"].(string)
			switch mode {
			case "local":
				os.WriteFile(m, []byte("changed"), 0600)
			case "remote":
				f.data[transferFiles[1]][271] ^= 1
			case "expired":
				s.plans[id].created = time.Now().Add(-11 * time.Minute)
			}
			if _, e := s.Upload(id); e == nil {
				t.Fatal("stale upload accepted")
			}
			if f.writes != 0 {
				t.Fatal("stale review wrote to console")
			}
		})
	}
}
func TestTransferDownloadAndValidation(t *testing.T) {
	s, f, c, _, _ := setupTransfer(t)
	r, e := s.Download(c)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range r["paths"].([]string) {
		b, e := os.ReadFile(p)
		if e != nil || !bytes.Equal(b, f.data[filepath.Base(p)]) {
			t.Fatal("download mismatch")
		}
	}
	if f.writes != 0 {
		t.Fatal("download wrote to console")
	}
	f.data[transferFiles[1]] = []byte("wrong format")
	if _, e = s.Check(c); e == nil {
		t.Fatal("unsupported pair accepted")
	}
	for _, mutate := range []func(*ftpConfig){func(c *ftpConfig) { c.Host = "" }, func(c *ftpConfig) { c.Port = 0 }, func(c *ftpConfig) { c.Directory = "/x/../y" }, func(c *ftpConfig) { c.Password = "x\r\nSTOR y" }} {
		bad := c
		mutate(&bad)
		if _, e = bad.validated(); e == nil {
			t.Fatal("bad config accepted")
		}
	}
}
