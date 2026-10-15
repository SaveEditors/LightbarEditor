// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 SaveEditors

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

var transferFiles = []string{"shutdown_lightbar.fbxd", "shutdown_ramp.gnf"}

const maxTransferFile = 32 << 20

type ftpConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	Directory string `json:"directory"`
}

func (c ftpConfig) validated() (ftpConfig, error) {
	c.Host = strings.TrimSpace(c.Host)
	if c.Host == "" || strings.ContainsAny(c.Host, "/\\\r\n\x00 @?#") || strings.Contains(c.Host, "://") {
		return c, fmt.Errorf("enter the PS5 IP address or hostname, without a URL or port")
	}
	if strings.Contains(c.Host, ":") && net.ParseIP(c.Host) == nil {
		return c, fmt.Errorf("enter the host without a port; use the separate Port field")
	}
	if c.Port < 1 || c.Port > 65535 {
		return c, fmt.Errorf("FTP port must be between 1 and 65535")
	}
	if strings.ContainsAny(c.User+c.Password, "\r\n\x00") {
		return c, fmt.Errorf("invalid FTP credentials")
	}
	if c.User == "" {
		c.User = "anonymous"
	}
	c.Directory = strings.TrimSpace(c.Directory)
	if !strings.HasPrefix(c.Directory, "/") || strings.ContainsAny(c.Directory, "\r\n\x00\\") {
		return c, fmt.Errorf("remote folder must be an absolute FTP path")
	}
	for _, part := range strings.Split(c.Directory, "/") {
		if part == ".." {
			return c, fmt.Errorf("remote folder cannot contain parent traversal")
		}
	}
	c.Directory = path.Clean(c.Directory)
	return c, nil
}

func (c ftpConfig) address() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) }
func assetHash(data []byte) string  { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }

type assetFTP interface {
	Read(string) ([]byte, error)
	Write(string, []byte) error
	Close()
}
type wireFTP struct{ c *ftp.ServerConn }

func dialAssetFTP(cfg ftpConfig) (assetFTP, error) {
	c, err := ftp.Dial(cfg.address(), ftp.DialWithDialFunc(func(network, address string) (net.Conn, error) {
		conn, err := net.DialTimeout(network, address, 15*time.Second)
		if err == nil {
			err = conn.SetDeadline(time.Now().Add(90 * time.Second))
			if err != nil {
				conn.Close()
			}
		}
		return conn, err
	}))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to FTP: %w", err)
	}
	if err = c.Login(cfg.User, cfg.Password); err != nil {
		c.Quit()
		return nil, fmt.Errorf("FTP login failed: %w", err)
	}
	return &wireFTP{c}, nil
}
func (w *wireFTP) Close() { _ = w.c.Quit() }
func (w *wireFTP) Read(remote string) ([]byte, error) {
	r, err := w.c.Retr(remote)
	if err != nil {
		return nil, err
	}
	_ = r.SetDeadline(time.Now().Add(45 * time.Second))
	data, readErr := io.ReadAll(io.LimitReader(r, maxTransferFile+1))
	closeErr := r.Close()
	if readErr != nil {
		return nil, readErr
	}
	if len(data) > maxTransferFile {
		return nil, fmt.Errorf("file exceeds the 32 MiB transfer limit")
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return data, nil
}
func (w *wireFTP) Write(remote string, data []byte) error {
	return w.c.Stor(remote, bytes.NewReader(data))
}

func validateTransferPair(data map[string][]byte) error {
	for _, name := range transferFiles {
		if len(data[name]) > maxTransferFile {
			return fmt.Errorf("%s exceeds the transfer limit", name)
		}
	}
	if _, err := ParseFBXD(data[transferFiles[0]]); err != nil {
		return fmt.Errorf("lightbar file is not supported: %w", err)
	}
	if _, err := ParseGNF(data[transferFiles[1]]); err != nil {
		return fmt.Errorf("ramp file is not supported: %w", err)
	}
	return nil
}
func readRemotePair(c assetFTP, cfg ftpConfig) (map[string][]byte, error) {
	data := map[string][]byte{}
	for _, name := range transferFiles {
		b, err := c.Read(path.Join(cfg.Directory, name))
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w; check the remote folder and FTP permissions", name, err)
		}
		data[name] = b
	}
	return data, validateTransferPair(data)
}
func pairInfo(data map[string][]byte) []map[string]any {
	files := []map[string]any{}
	for _, name := range transferFiles {
		files = append(files, map[string]any{"name": name, "bytes": len(data[name]), "sha256": assetHash(data[name])})
	}
	return files
}

type uploadPlan struct {
	config        ftpConfig
	created       time.Time
	sources       map[string]string
	before, after map[string][]byte
	backup        string
}
type transferService struct {
	root  string
	mu    sync.Mutex
	plans map[string]*uploadPlan
	dial  func(ftpConfig) (assetFTP, error)
}

func newTransferService(root string) *transferService {
	return &transferService{root: root, plans: map[string]*uploadPlan{}, dial: dialAssetFTP}
}
func (s *transferService) lock() error {
	if !s.mu.TryLock() {
		return fmt.Errorf("another FTP operation is still running")
	}
	return nil
}
func (s *transferService) snapshot(kind string, cfg ftpConfig, data map[string][]byte) (string, error) {
	root := filepath.Join(s.root, kind)
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(root, time.Now().UTC().Format("20060102-150405")+"-")
	if err != nil {
		return "", err
	}
	for _, name := range transferFiles {
		p := filepath.Join(dir, name)
		if err = os.WriteFile(p, data[name], 0644); err != nil {
			return "", err
		}
		b, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(b, data[name]) {
			return "", fmt.Errorf("local backup verification failed for %s", name)
		}
	}
	manifest := map[string]any{"created": time.Now().UTC(), "server": cfg.address(), "remote_folder": cfg.Directory, "files": pairInfo(data), "kind": kind}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0644); err != nil {
		return "", err
	}
	return dir, nil
}
func (s *transferService) Check(raw ftpConfig) (map[string]any, error) {
	if err := s.lock(); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	cfg, err := raw.validated()
	if err != nil {
		return nil, err
	}
	c, err := s.dial(cfg)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	data, err := readRemotePair(c, cfg)
	if err != nil {
		return nil, err
	}
	return map[string]any{"server": cfg.address(), "directory": cfg.Directory, "files": pairInfo(data)}, nil
}
func (s *transferService) Download(raw ftpConfig) (map[string]any, error) {
	if err := s.lock(); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	cfg, err := raw.validated()
	if err != nil {
		return nil, err
	}
	c, err := s.dial(cfg)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	data, err := readRemotePair(c, cfg)
	if err != nil {
		return nil, err
	}
	dir, err := s.snapshot("downloads", cfg, data)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, name := range transferFiles {
		files = append(files, filepath.Join(dir, name))
	}
	return map[string]any{"directory": dir, "paths": files, "files": pairInfo(data)}, nil
}
func (s *transferService) Prepare(raw ftpConfig, mesh, ramp string) (map[string]any, error) {
	if err := s.lock(); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	cfg, err := raw.validated()
	if err != nil {
		return nil, err
	}
	data := map[string][]byte{}
	sources := map[string]string{transferFiles[0]: mesh, transferFiles[1]: ramp}
	for _, name := range transferFiles {
		fi, err := os.Stat(sources[name])
		if err != nil {
			return nil, err
		}
		if fi.Size() > maxTransferFile {
			return nil, fmt.Errorf("%s exceeds the transfer limit", name)
		}
		data[name], err = os.ReadFile(sources[name])
		if err != nil {
			return nil, err
		}
	}
	if err = validateTransferPair(data); err != nil {
		return nil, err
	}
	c, err := s.dial(cfg)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	before, err := readRemotePair(c, cfg)
	if err != nil {
		return nil, err
	}
	dir, err := s.snapshot("console-backups", cfg, before)
	if err != nil {
		return nil, err
	}
	key := make([]byte, 16)
	if _, err = rand.Read(key); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(key)
	for k, p := range s.plans {
		if time.Since(p.created) > 10*time.Minute {
			delete(s.plans, k)
		}
	}
	if len(s.plans) >= 8 {
		return nil, fmt.Errorf("too many pending transfer reviews; finish one or restart the editor")
	}
	s.plans[id] = &uploadPlan{cfg, time.Now(), sources, before, data, dir}
	return map[string]any{"plan": id, "server": cfg.address(), "directory": cfg.Directory, "backup": dir, "files": pairInfo(data), "previous": pairInfo(before), "expires_minutes": 10}, nil
}
func (s *transferService) Upload(id string) (map[string]any, error) {
	if err := s.lock(); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	p, ok := s.plans[id]
	if !ok {
		return nil, fmt.Errorf("prepare a new upload review first")
	}
	delete(s.plans, id)
	if time.Since(p.created) > 10*time.Minute {
		return nil, fmt.Errorf("upload review expired; prepare it again")
	}
	for _, name := range transferFiles {
		b, err := os.ReadFile(p.sources[name])
		if err != nil || !bytes.Equal(b, p.after[name]) {
			return nil, fmt.Errorf("saved local files changed; prepare a new upload review")
		}
	}
	c, err := s.dial(p.config)
	if err != nil {
		return nil, err
	}
	before, err := readRemotePair(c, p.config)
	if err != nil {
		c.Close()
		return nil, err
	}
	for _, name := range transferFiles {
		if !bytes.Equal(before[name], p.before[name]) {
			c.Close()
			return nil, fmt.Errorf("console files changed since the backup; prepare a new upload review")
		}
	}
	if err = writeTransferJournal(p.backup, map[string]any{"status": "upload starting", "files": pairInfo(p.after)}); err != nil {
		c.Close()
		return nil, fmt.Errorf("cannot save the local transfer report; no console files were written: %w", err)
	}
	attempted := []string{}
	for _, name := range transferFiles {
		attempted = append(attempted, name)
		err = c.Write(path.Join(p.config.Directory, name), p.after[name])
		if err == nil {
			var b []byte
			b, err = c.Read(path.Join(p.config.Directory, name))
			if err == nil && !bytes.Equal(b, p.after[name]) {
				err = fmt.Errorf("downloaded bytes differ after upload")
			}
		}
		if err != nil {
			break
		}
	}
	c.Close()
	if err != nil {
		cause := err
		failures := []string{}
		rollback, connectErr := s.dial(p.config)
		if connectErr != nil {
			failures = append(failures, "could not reconnect for restoration")
		} else {
			for _, name := range attempted {
				e := rollback.Write(path.Join(p.config.Directory, name), p.before[name])
				if e == nil {
					var b []byte
					b, e = rollback.Read(path.Join(p.config.Directory, name))
					if e == nil && !bytes.Equal(b, p.before[name]) {
						e = fmt.Errorf("restored bytes differ")
					}
				}
				if e != nil {
					failures = append(failures, name+": "+e.Error())
				}
			}
			rollback.Close()
		}
		journal := map[string]any{"status": "upload failed", "error": cause.Error(), "restoration_errors": failures}
		_ = writeTransferJournal(p.backup, journal)
		if len(failures) > 0 {
			return nil, fmt.Errorf("upload failed: %v. Automatic restoration incomplete (%s). Original files: %s", cause, strings.Join(failures, "; "), p.backup)
		}
		return nil, fmt.Errorf("upload failed: %v. Changed files were restored and verified. Backup: %s", cause, p.backup)
	}
	if err = writeTransferJournal(p.backup, map[string]any{"status": "uploaded and readback verified", "files": pairInfo(p.after)}); err != nil {
		return nil, fmt.Errorf("both console files were uploaded and verified, but the local transfer report could not be saved: %w", err)
	}
	return map[string]any{"ok": true, "files": pairInfo(p.after), "backup": p.backup, "server": p.config.address(), "directory": p.config.Directory}, nil
}
func writeTransferJournal(dir string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "transfer-result.json"), b, 0644)
}
