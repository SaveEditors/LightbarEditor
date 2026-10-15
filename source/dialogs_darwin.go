// SPDX-License-Identifier: GPL-3.0-only
// macOS port additions, 2026
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var dialogLock sync.Mutex
var dialogInput = bufio.NewReader(os.Stdin)

func nativeDialog(args ...string) ([]string, error) {
	if os.Getenv("LIGHTBAR_NATIVE_BRIDGE") == "1" {
		dialogLock.Lock()
		defer dialogLock.Unlock()
		b, _ := json.Marshal(args)
		fmt.Println("LIGHTBAR_DIALOG\t" + string(b))
		line, e := dialogInput.ReadBytes('\n')
		if e != nil {
			return nil, e
		}
		var paths []string
		e = json.Unmarshal(line, &paths)
		return paths, e
	}
	p := os.Getenv("LIGHTBAR_NATIVE_HOST")
	if p == "" {
		exe, _ := os.Executable()
		p = filepath.Join(filepath.Dir(exe), "LightbarEditor")
	}
	b, e := exec.Command(p, args...).Output()
	if e != nil {
		return nil, e
	}
	var paths []string
	e = json.Unmarshal(b, &paths)
	return paths, e
}
func OpenFileDialog(multi bool) ([]string, error) {
	flag := "single"
	if multi {
		flag = "multi"
	}
	return nativeDialog("--open-dialog", flag)
}
func SaveFileDialog(suggested, dir string) (string, error) {
	p, e := nativeDialog("--save-dialog", suggested, dir)
	if e != nil {
		return "", e
	}
	if len(p) == 0 {
		return "", nil
	}
	return p[0], nil
}
func OpenBrowser(url string) error     { return exec.Command("open", url).Run() }
func RevealInFolder(path string) error { return exec.Command("open", "-R", path).Run() }
