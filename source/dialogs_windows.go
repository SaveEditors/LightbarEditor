// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 SaveEditors

//go:build windows

package main

import (
	"encoding/base64"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
)

func runPS(script string) (string, error) {
	u := utf16.Encode([]rune(script))
	b := make([]byte, len(u)*2)
	for i, c := range u {
		b[i*2] = byte(c)
		b[i*2+1] = byte(c >> 8)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", base64.StdEncoding.EncodeToString(b))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

const psFilter = "Editor files (*.fbxd;*.gnf)|*.fbxd;*.gnf|Mesh (*.fbxd)|*.fbxd|Texture (*.gnf)|*.gnf|All files (*.*)|*.*"

func OpenFileDialog(multi bool) ([]string, error) {
	ms := "$false"
	if multi {
		ms = "$true"
	}
	out, err := runPS(`Add-Type -AssemblyName System.Windows.Forms
[Console]::OutputEncoding=[Text.Encoding]::UTF8
$o=New-Object System.Windows.Forms.Form -Property @{TopMost=$true}
$d=New-Object System.Windows.Forms.OpenFileDialog
$d.Filter=` + psQuote(psFilter) + `
$d.Multiselect=` + ms + `
if($d.ShowDialog($o) -eq [System.Windows.Forms.DialogResult]::OK){ $d.FileNames -join "|" }`)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "|"), nil
}

func SaveFileDialog(suggested, dir string) (string, error) {
	return runPS(`Add-Type -AssemblyName System.Windows.Forms
[Console]::OutputEncoding=[Text.Encoding]::UTF8
$o=New-Object System.Windows.Forms.Form -Property @{TopMost=$true}
$d=New-Object System.Windows.Forms.SaveFileDialog
$d.Filter=` + psQuote(psFilter) + `
$d.FilterIndex=4
$d.FileName=` + psQuote(suggested) + `
if(` + psQuote(dir) + ` -ne ''){ $d.InitialDirectory=` + psQuote(dir) + ` }
$d.OverwritePrompt=$true
if($d.ShowDialog($o) -eq [System.Windows.Forms.DialogResult]::OK){ $d.FileName }`)
}

func OpenBrowser(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

// RevealInFolder opens Explorer with the file selected.
func RevealInFolder(path string) error {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	return cmd.Start()
}
