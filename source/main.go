// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 SaveEditors

package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const appVersion = "1.7.0"

//go:embed ui/index.html
var indexHTML string

//go:embed ui/fonts/*.woff2
var uiFonts embed.FS

const usage = `Lightbar Editor ` + appVersion + ` – edit .fbxd light strips, .gnf ramp textures

  LightbarEditor.exe                         open the visual editor in your browser
  LightbarEditor.exe file1 file2 ...         open the editor with these files (or drag files onto the .exe)

  LightbarEditor.exe info   <file>           show what's inside (.fbxd / .gnf)

 .fbxd light strips
  LightbarEditor.exe color  <in.fbxd> <out.fbxd> --color #ff0000 [--end #0000ff] [--strip 0|1]
                                             [--brightness 1.5] [--solid]
  LightbarEditor.exe decode <in.fbxd> <out.json>      editable JSON (one vertex per line)
  LightbarEditor.exe encode <in.json> <out.fbxd>      rebuild from JSON
  LightbarEditor.exe obj    <in.fbxd> <out.obj>       for viewing in Blender

 .gnf ramp texture (BC7)
  LightbarEditor.exe topng   <in.gnf> <out.png>
  LightbarEditor.exe frompng <in.gnf> <edited.png> <out.gnf>   (PNG must be the same size)

Saving over an existing file always keeps the first original as <file>.bak.
`

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "-h", "--help", "help", "/?":
			fmt.Print(usage)
			return
		case "info", "color", "decode", "encode", "obj", "topng", "frompng", "verify":
			if err := runCLI(args); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
			return
		}
	}
	for _, a := range args {
		if _, err := os.Stat(a); err != nil {
			fmt.Fprintf(os.Stderr, "'%s' is not a command or an existing file.\n\n", a)
			fmt.Print(usage)
			os.Exit(2)
		}
	}
	if err := runServer(args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Println("Press Enter to close.")
		bufio.NewReader(os.Stdin).ReadString('\n')
		os.Exit(1)
	}
}

// ---------------- CLI ----------------

type cliArgs struct {
	pos   []string
	flags map[string]string
}

func parseCLI(a []string, boolFlags ...string) cliArgs {
	r := cliArgs{flags: map[string]string{}}
	isBool := map[string]bool{}
	for _, b := range boolFlags {
		isBool[b] = true
	}
	for i := 0; i < len(a); i++ {
		if strings.HasPrefix(a[i], "--") {
			name := strings.TrimPrefix(a[i], "--")
			if k, v, ok := strings.Cut(name, "="); ok {
				r.flags[k] = v
			} else if isBool[name] {
				r.flags[name] = "1"
			} else if i+1 < len(a) {
				r.flags[name] = a[i+1]
				i++
			} else {
				r.flags[name] = ""
			}
		} else {
			r.pos = append(r.pos, a[i])
		}
	}
	return r
}

func (c cliArgs) need(n int) error {
	if len(c.pos) < n {
		return fmt.Errorf("missing arguments – run with --help")
	}
	return nil
}

func (c cliArgs) num(name string, def float64) (float64, error) {
	v, ok := c.flags[name]
	if !ok {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("--%s: %v", name, err)
	}
	return f, nil
}

func writeOut(path string, data []byte) error {
	bak, err := SafeWrite(path, data)
	if err != nil {
		return err
	}
	fmt.Printf("saved %s (%d bytes)\n", path, len(data))
	if bak != "" {
		fmt.Printf("backup kept as %s\n", bak)
	}
	return nil
}

func runCLI(a []string) error {
	cmd := strings.ToLower(a[0])
	c := parseCLI(a[1:], "solid")
	if err := c.need(1); err != nil {
		return err
	}
	in, err := os.ReadFile(c.pos[0])
	if err != nil {
		return err
	}
	switch cmd {
	case "info", "verify":
		return printInfo(c.pos[0], in)
	case "decode", "obj", "color":
		m, err := ParseFBXD(in)
		if err != nil {
			return err
		}
		if err := c.need(2); err != nil {
			return err
		}
		switch cmd {
		case "decode":
			return os.WriteFile(c.pos[1], m.ToJSON(), 0644)
		case "obj":
			return m.WriteOBJ(c.pos[1])
		}
		op := ColorOp{Strand: -1, Brightness: 1, A: [3]float64{1, 1, 1}}
		if v, ok := c.flags["color"]; ok {
			if op.A, err = ParseColor(v); err != nil {
				return err
			}
		}
		if v, ok := c.flags["end"]; ok {
			if op.B, err = ParseColor(v); err != nil {
				return err
			}
			op.Gradient = true
		}
		if v, ok := c.flags["strip"]; ok {
			if op.Strand, err = strconv.Atoi(v); err != nil {
				return fmt.Errorf("--strip: %v", err)
			}
		}
		if op.Brightness, err = c.num("brightness", 1); err != nil {
			return err
		}
		op.Solid = c.flags["solid"] != ""
		if err := m.ApplyColor(op); err != nil {
			return err
		}
		out, err := m.Patched(m.Pos, m.Col)
		if err != nil {
			return err
		}
		return writeOut(c.pos[1], out)
	case "encode":
		if err := c.need(2); err != nil {
			return err
		}
		out, err := FBXDFromJSON(in)
		if err != nil {
			return err
		}
		return writeOut(c.pos[1], out)
	case "topng", "frompng":
		g, err := ParseGNF(in)
		if err != nil {
			return err
		}
		if cmd == "topng" {
			if err := c.need(2); err != nil {
				return err
			}
			return writePNG(c.pos[1], g.RGBA(), g.Width, g.Height)
		}
		if err := c.need(3); err != nil {
			return err
		}
		rgba, w, h, err := readPNG(c.pos[1])
		if err != nil {
			return err
		}
		if w != g.Width || h != g.Height {
			return fmt.Errorf("PNG is %dx%d but the texture is %dx%d", w, h, g.Width, g.Height)
		}
		out, err := g.WithRGBA(rgba)
		if err != nil {
			return err
		}
		g2, err := ParseGNF(out)
		if err != nil {
			return fmt.Errorf("verification failed: %v", err)
		}
		fmt.Printf("BC7 encode quality: %.1f dB PSNR\n", PSNR(rgba, g2.RGBA()))
		return writeOut(c.pos[2], out)

	}
	return fmt.Errorf("unknown command %s", cmd)
}

func printInfo(path string, in []byte) error {
	switch kindOf(path, in) {
	case "fbxd":
		m, err := ParseFBXD(in)
		if err != nil {
			return err
		}
		fmt.Printf(".fbxd light-strip mesh: %d vertices, %d triangles\n", m.VC, m.IC/3)
		for i, s := range m.Strands() {
			mx := float32(0)
			for _, v := range s {
				for c := 0; c < 3; c++ {
					if m.Col[v*4+c] > mx {
						mx = m.Col[v*4+c]
					}
				}
			}
			fmt.Printf("  strip %d: vertices %d, length %.1f, peak intensity %.3f\n", i, len(s), m.UV[s[len(s)-1]*2+1], mx)
		}
		fmt.Println("file structure OK")
	case "gnf":
		g, err := ParseGNF(in)
		if err != nil {
			return err
		}
		fmt.Printf(".gnf texture v%d: %dx%d BC7 (format code %d), linear, %d bytes of pixel data\n", g.Version, g.Width, g.Height, g.Format, g.DataSize)
		fmt.Println("file structure OK")
	default:
		return fmt.Errorf("unrecognised file type")
	}
	return nil
}

func writePNG(path string, rgba []byte, w, h int) error {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, rgba)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0644)
}

func readPNG(path string) ([]byte, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%s: %v", path, err)
	}
	b := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Bounds(), img, b.Min, draw.Src)
	return n.Pix, b.Dx(), b.Dy(), nil
}

// ---------------- editor server ----------------

type apiErr struct {
	Error string `json:"error"`
}

func runServer(files []string) error {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("cannot locate your settings folder: %w", err)
	}
	dataDir := filepath.Join(configRoot, "LightbarEditor")
	if err := os.MkdirAll(filepath.Join(dataDir, "presets"), 0755); err != nil {
		return fmt.Errorf("cannot create your settings folder: %w", err)
	}
	absFiles := []string{}
	for _, f := range files {
		if p, err := filepath.Abs(f); err == nil {
			absFiles = append(absFiles, p)
		}
	}
	tokB := make([]byte, 16)
	rand.Read(tokB)
	token := hex.EncodeToString(tokB)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	host := fmt.Sprintf("127.0.0.1:%d", port)
	quit := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || (r.Host != host && r.Host != fmt.Sprintf("localhost:%d", port)) {
			// Consume small rejected requests before replying. Closing a Windows
			// socket with unread POST data can reset it before the 404 arrives.
			if r.Body != nil {
				_, _ = io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, 4096))
			}
			http.NotFound(w, r)
			return
		}
		cfg, _ := json.Marshal(map[string]any{"token": token, "version": appVersion})
		page := strings.Replace(indexHTML, "/*__SERVER_CONFIG__*/null", string(cfg), 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte(page))
	})
	mux.HandleFunc("/fonts/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/fonts/")
		data, err := uiFonts.ReadFile("ui/fonts/" + name)
		if err != nil || strings.Contains(name, "/") || (r.Host != host && r.Host != fmt.Sprintf("localhost:%d", port)) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "font/woff2")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write(data)
	})
	api := func(path string, h func(req map[string]any) (any, error)) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodPost || r.Header.Get("X-Token") != token || (r.Host != host && r.Host != fmt.Sprintf("localhost:%d", port)) {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(apiErr{"forbidden"})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				json.NewEncoder(w).Encode(apiErr{"bad request: " + err.Error()})
				return
			}
			var res any
			var handlerErr error
			func() {
				defer func() {
					if p := recover(); p != nil {
						handlerErr = fmt.Errorf("internal error: %v", p)
						res = nil
					}
				}()
				res, handlerErr = h(req)
			}()
			if handlerErr != nil {
				json.NewEncoder(w).Encode(apiErr{handlerErr.Error()})
				return
			}
			json.NewEncoder(w).Encode(res)
		})
	}

	str := func(m map[string]any, k string) string { s, _ := m[k].(string); return s }
	f32s := func(m map[string]any, k string) []float32 {
		arr, _ := m[k].([]any)
		out := make([]float32, len(arr))
		for i, v := range arr {
			f, _ := v.(float64)
			out[i] = float32(f)
		}
		return out
	}
	dstOf := func(req map[string]any) string {
		if d := str(req, "dst"); d != "" {
			return d
		}
		return str(req, "src")
	}
	saveResult := func(dst string, data []byte) (map[string]any, error) {
		bak, err := SafeWrite(dst, data)
		if err != nil {
			return nil, err
		}
		abs, _ := filepath.Abs(dst)
		return map[string]any{"ok": true, "path": abs, "name": filepath.Base(abs), "backup": bak, "bytes": len(data)}, nil
	}

	transfers := newTransferService(dataDir)
	ftpRequest := func(req map[string]any) (ftpConfig, error) {
		var cfg ftpConfig
		b, err := json.Marshal(req["config"])
		if err == nil {
			err = json.Unmarshal(b, &cfg)
		}
		return cfg, err
	}
	api("/api/ftp/check", func(req map[string]any) (any, error) {
		cfg, err := ftpRequest(req)
		if err != nil {
			return nil, err
		}
		return transfers.Check(cfg)
	})
	api("/api/ftp/download", func(req map[string]any) (any, error) {
		cfg, err := ftpRequest(req)
		if err != nil {
			return nil, err
		}
		return transfers.Download(cfg)
	})
	api("/api/ftp/prepare", func(req map[string]any) (any, error) {
		cfg, err := ftpRequest(req)
		if err != nil {
			return nil, err
		}
		return transfers.Prepare(cfg, str(req, "mesh"), str(req, "ramp"))
	})
	api("/api/ftp/upload", func(req map[string]any) (any, error) { return transfers.Upload(str(req, "plan")) })
	api("/api/args", func(req map[string]any) (any, error) {
		return map[string]any{"files": absFiles, "os": runtime.GOOS}, nil
	})
	api("/api/open-dialog", func(req map[string]any) (any, error) {
		paths, err := OpenFileDialog(true)
		if err != nil {
			return nil, err
		}
		return map[string]any{"paths": paths}, nil
	})
	api("/api/save-dialog", func(req map[string]any) (any, error) {
		p, err := SaveFileDialog(str(req, "suggested"), str(req, "dir"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"path": p}, nil
	})
	api("/api/load", func(req map[string]any) (any, error) {
		p, err := filepath.Abs(strings.Trim(strings.TrimSpace(str(req, "path")), `"`))
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		_, bakErr := os.Stat(p + ".bak")
		res := map[string]any{"path": p, "name": filepath.Base(p), "dir": filepath.Dir(p), "size": len(data), "hasBackup": bakErr == nil}
		switch kindOf(p, data) {
		case "fbxd":
			m, err := ParseFBXD(data)
			if err != nil {
				return nil, err
			}
			res["kind"] = "fbxd"
			res["vc"], res["ic"] = m.VC, m.IC
			res["pos"], res["uv"], res["col"] = m.Pos, m.UV, m.Col
			res["strands"] = m.Strands()
		case "gnf":
			g, err := ParseGNF(data)
			if err != nil {
				return nil, err
			}
			res["kind"] = "gnf"
			res["width"], res["height"], res["format"] = g.Width, g.Height, g.Format
			res["rgba"] = base64.StdEncoding.EncodeToString(g.RGBA())
		default:
			return nil, fmt.Errorf("%s is not a .fbxd or .gnf file this editor understands", filepath.Base(p))
		}
		return res, nil
	})
	api("/api/save/fbxd", func(req map[string]any) (any, error) {
		data, err := os.ReadFile(str(req, "src"))
		if err != nil {
			return nil, err
		}
		m, err := ParseFBXD(data)
		if err != nil {
			return nil, err
		}
		out, err := m.Patched(f32s(req, "pos"), f32s(req, "col"))
		if err != nil {
			return nil, err
		}
		return saveResult(dstOf(req), out)
	})
	api("/api/save/gnf", func(req map[string]any) (any, error) {
		data, err := os.ReadFile(str(req, "src"))
		if err != nil {
			return nil, err
		}
		g, err := ParseGNF(data)
		if err != nil {
			return nil, err
		}
		rgba, err := base64.StdEncoding.DecodeString(str(req, "rgba"))
		if err != nil {
			return nil, err
		}
		out, err := g.WithRGBA(rgba)
		if err != nil {
			return nil, err
		}
		g2, err := ParseGNF(out)
		if err != nil {
			return nil, fmt.Errorf("verification failed: %v", err)
		}
		dec := g2.RGBA()
		res, err := saveResult(dstOf(req), out)
		if err != nil {
			return nil, err
		}
		res["psnr"] = PSNR(rgba, dec)
		res["rgba"] = base64.StdEncoding.EncodeToString(dec)
		return res, nil
	})
	api("/api/export", func(req map[string]any) (any, error) {
		data, err := base64.StdEncoding.DecodeString(str(req, "data"))
		if err != nil {
			return nil, err
		}
		dst := str(req, "dst")
		if dst == "" {
			return nil, fmt.Errorf("no destination path")
		}
		if !strings.EqualFold(filepath.Ext(dst), ".png") {
			return nil, fmt.Errorf("image export requires a .png destination")
		}
		if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("invalid PNG image: %v", err)
		}
		return saveResult(dst, data)
	})
	api("/api/siblings", func(req map[string]any) (any, error) {
		dir := filepath.Dir(str(req, "path"))
		ents, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, e := range ents {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if !e.IsDir() && (ext == ".fbxd" || ext == ".gnf") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
		return map[string]any{"files": files}, nil
	})
	presetDir := func() string { return filepath.Join(dataDir, "presets") }
	safeName := func(n string) string {
		n = strings.Map(func(r rune) rune {
			if strings.ContainsRune(`<>:"/\\|?*`, r) || r < 32 {
				return '_'
			}
			return r
		}, strings.TrimSpace(n))
		if n == "" {
			n = "preset"
		}
		if len(n) > 60 {
			n = n[:60]
		}
		return n
	}
	settingsPath := func() string { return filepath.Join(dataDir, "settings.json") }
	api("/api/settings/get", func(req map[string]any) (any, error) {
		var v any
		if b, err := os.ReadFile(settingsPath()); err == nil {
			json.Unmarshal(b, &v)
		}
		return map[string]any{"settings": v}, nil
	})
	api("/api/settings/set", func(req map[string]any) (any, error) {
		b, err := json.MarshalIndent(req["settings"], "", "  ")
		if err != nil {
			return nil, err
		}
		tmp := settingsPath() + ".tmp"
		if err := os.WriteFile(tmp, b, 0644); err != nil {
			return nil, fmt.Errorf("cannot save settings: %w", err)
		}
		if err := os.Rename(tmp, settingsPath()); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})
	api("/api/reveal", func(req map[string]any) (any, error) {
		p := str(req, "path")
		if _, err := os.Stat(p); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, RevealInFolder(p)
	})
	api("/api/presets/list", func(req map[string]any) (any, error) {
		dir := presetDir()
		ents, _ := os.ReadDir(dir)
		var out []any
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var v any
			if json.Unmarshal(b, &v) == nil {
				out = append(out, map[string]any{"file": e.Name(), "preset": v})
			}
		}
		return map[string]any{"presets": out, "dir": dir}, nil
	})
	api("/api/presets/save", func(req map[string]any) (any, error) {
		dir := presetDir()
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		b, err := json.MarshalIndent(req["preset"], "", "  ")
		if err != nil {
			return nil, err
		}
		name := safeName(str(req, "name")) + ".json"
		if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "file": name, "dir": dir}, nil
	})
	api("/api/presets/delete", func(req map[string]any) (any, error) {
		name := filepath.Base(str(req, "file"))
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			return nil, fmt.Errorf("not a preset file")
		}
		return map[string]any{"ok": true}, os.Remove(filepath.Join(presetDir(), name))
	})
	api("/api/restore", func(req map[string]any) (any, error) {
		path := str(req, "path")
		data, err := os.ReadFile(path + ".bak")
		if err != nil {
			return nil, err
		}
		switch kindOf(path, data) {
		case "fbxd":
			_, err = ParseFBXD(data)
		case "gnf":
			_, err = ParseGNF(data)
		default:
			return nil, fmt.Errorf("backup is not a supported lightbar mesh or ramp texture")
		}
		if err != nil {
			return nil, fmt.Errorf("invalid backup: %w", err)
		}
		if err := RestoreBackup(path); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})
	api("/api/quit", func(req map[string]any) (any, error) {
		go func() { time.Sleep(200 * time.Millisecond); close(quit) }()
		return map[string]any{"ok": true}, nil
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	url := "http://" + host + "/"
	fmt.Println("Lightbar Editor", appVersion)
	fmt.Println("Editor running at", url)
	fmt.Println("Keep this window open while you edit. Close it (or press Quit in the editor) to exit.")
	if os.Getenv("LIGHTBAR_NO_BROWSER") == "" {
		if err := OpenBrowser(url); err != nil {
			fmt.Println("Could not open the browser automatically – open the address above yourself.")
		}
	}
	<-quit
	srv.Close()
	return nil
}
