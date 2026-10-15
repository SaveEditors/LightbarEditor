// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 SaveEditors

//go:build !windows && !darwin

package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

func OpenFileDialog(multi bool) ([]string, error) {
	return nil, fmt.Errorf("file dialogs are only available on Windows – type the path instead")
}

func SaveFileDialog(suggested, dir string) (string, error) {
	return "", fmt.Errorf("file dialogs are only available on Windows – type the path instead")
}

func OpenBrowser(url string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}

// RevealInFolder opens the file's folder in the system file manager.
func RevealInFolder(path string) error {
	dir := filepath.Dir(path)
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", path).Start()
	}
	return exec.Command("xdg-open", dir).Start()
}
