// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 SaveEditors

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SafeWrite writes data to path:
//  1. if path exists and path+".bak" does not, the current file is copied to .bak
//     (so the very first original is always kept)
//  2. data goes to a temp file in the same folder, is synced, read back and compared
//  3. the temp file atomically replaces the target
func SafeWrite(path string, data []byte) (backup string, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if fi, err := os.Stat(abs); err == nil {
		if fi.IsDir() {
			return "", fmt.Errorf("%s is a folder", abs)
		}
		bak := abs + ".bak"
		if _, err := os.Stat(bak); os.IsNotExist(err) {
			if err := copyFile(abs, bak); err != nil {
				return "", fmt.Errorf("could not create backup %s: %v", bak, err)
			}
			backup = bak
		}
	}
	tmp := abs + ".tmp-save"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return backup, fmt.Errorf("cannot write in %s: %v", filepath.Dir(abs), err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return backup, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return backup, err
	}
	f.Close()
	back, err := os.ReadFile(tmp)
	if err != nil || string(back) != string(data) {
		os.Remove(tmp)
		return backup, fmt.Errorf("write verification failed (disk full?)")
	}
	if err := os.Rename(tmp, abs); err != nil {
		os.Remove(tmp)
		return backup, fmt.Errorf("could not replace %s (is it open in another program?): %v", abs, err)
	}
	return backup, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// RestoreBackup copies path.bak back over path.
func RestoreBackup(path string) error {
	bak := path + ".bak"
	if _, err := os.Stat(bak); err != nil {
		return fmt.Errorf("no backup found (%s)", bak)
	}
	data, err := os.ReadFile(bak)
	if err != nil {
		return err
	}
	tmp := path + ".tmp-restore"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func kindOf(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case len(data) >= 4 && string(data[:4]) == "GNF ":
		return "gnf"
	case ext == ".fbxd":
		return "fbxd"
	case ext == ".gnf":
		return "gnf"
	}
	if _, err := ParseFBXD(data); err == nil {
		return "fbxd"
	}
	return ""
}
