package server

import (
	"encoding/base64"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/WuErPing/solo/protocol"
)

func (s *Session) handleFileExplorer(m *protocol.FileExplorerRequest) {
	baseDir := m.Cwd
	if baseDir == "" {
		err := "cwd is required"
		s.sendMessage(protocol.NewSessionMessage(&protocol.FileExplorerResponse{
			Type: "file_explorer_response",
			Payload: protocol.FileExplorerResponsePayload{
				Cwd:       m.Cwd,
				Path:      "",
				Mode:      m.Mode,
				Error:     &err,
				RequestID: m.RequestID,
			},
		}))
		return
	}

	reqPath := baseDir
	if m.Path != nil && *m.Path != "" {
		reqPath = *m.Path
		if !filepath.IsAbs(reqPath) {
			reqPath = filepath.Join(baseDir, reqPath)
		}
	}

	// Clean and resolve the path
	reqPath = filepath.Clean(reqPath)

	payload := protocol.FileExplorerResponsePayload{
		Cwd:       baseDir,
		Path:      reqPath,
		Mode:      m.Mode,
		RequestID: m.RequestID,
	}

	go func() {
		switch m.Mode {
		case "list":
			s.handleFileExplorerList(reqPath, &payload)
		case "file":
			s.handleFileExplorerFile(reqPath, &payload)
		default:
			err := "unsupported mode: " + m.Mode
			payload.Error = &err
		}

		s.sendMessage(protocol.NewSessionMessage(&protocol.FileExplorerResponse{
			Type:    "file_explorer_response",
			Payload: payload,
		}))
	}()
}

func (s *Session) handleFileExplorerList(dirPath string, payload *protocol.FileExplorerResponsePayload) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		errMsg := err.Error()
		payload.Error = &errMsg
		return
	}

	dir := &protocol.FileExplorerDirectory{
		Path:    relPath(payload.Cwd, dirPath),
		Entries: make([]protocol.FileExplorerEntry, 0, len(entries)),
	}

	// Concurrently stat entries to avoid serial syscall overhead on macOS,
	// where DirEntry.Info() may trigger an lstat per file.
	const maxWorkers = 32
	type entryResult struct {
		index int
		entry protocol.FileExplorerEntry
	}
	results := make(chan entryResult, len(entries))
	workCh := make(chan struct {
		idx int
		e   os.DirEntry
	}, len(entries))
	var wg sync.WaitGroup

	for i, entry := range entries {
		workCh <- struct {
			idx int
			e   os.DirEntry
		}{idx: i, e: entry}
	}
	close(workCh)

	workerCount := maxWorkers
	if len(entries) < workerCount {
		workerCount = len(entries)
	}
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range workCh {
				info, err := item.e.Info()
				if err != nil {
					continue
				}
				kind := "file"
				if item.e.IsDir() {
					kind = "directory"
				}
				absEntryPath := filepath.Join(dirPath, item.e.Name())
				results <- entryResult{
					index: item.idx,
					entry: protocol.FileExplorerEntry{
						Name:       item.e.Name(),
						Path:       relPath(payload.Cwd, absEntryPath),
						Kind:       kind,
						Size:       info.Size(),
						ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
					},
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results and maintain original order
	ordered := make([]protocol.FileExplorerEntry, len(entries))
	for r := range results {
		ordered[r.index] = r.entry
	}
	for _, e := range ordered {
		if e.Name != "" {
			dir.Entries = append(dir.Entries, e)
		}
	}

	payload.Directory = dir
}

func (s *Session) handleFileExplorerFile(filePath string, payload *protocol.FileExplorerResponsePayload) {
	info, err := os.Stat(filePath)
	if err != nil {
		errMsg := err.Error()
		payload.Error = &errMsg
		return
	}

	if info.IsDir() {
		errMsg := "path is a directory, use mode=list"
		payload.Error = &errMsg
		return
	}

	// Detect MIME type
	ext := filepath.Ext(filePath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Read content (limit to 10MB)
	const maxFileSize = 10 * 1024 * 1024
	var data []byte
	if info.Size() <= maxFileSize {
		data, err = os.ReadFile(filePath)
		if err != nil {
			errMsg := err.Error()
			payload.Error = &errMsg
			return
		}
	}

	// Determine kind: prefer content-based detection over MIME type
	kind := "binary"
	encoding := "base64"
	if isTextMimeType(mimeType) || isTextContent(data) {
		kind = "text"
		encoding = "utf-8"
		if mimeType == "application/octet-stream" {
			mimeType = "text/plain"
		}
	} else if strings.HasPrefix(mimeType, "image/") {
		kind = "image"
	}

	file := &protocol.FileExplorerFile{
		Path:       relPath(payload.Cwd, filePath),
		Kind:       kind,
		Encoding:   encoding,
		MimeType:   &mimeType,
		Size:       info.Size(),
		ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
	}

	if data != nil {
		if kind == "text" {
			content := string(data)
			file.Content = &content
		} else {
			content := base64.StdEncoding.EncodeToString(data)
			file.Content = &content
		}
	}

	payload.File = file
}

func (s *Session) handleProjectIcon(m *protocol.ProjectIconRequest) {
	payload := protocol.ProjectIconResponsePayload{
		Cwd:       m.Cwd,
		RequestID: m.RequestID,
	}

	// Look for common icon files in the project root
	iconFiles := []string{
		"icon.png", "icon.jpg", "icon.jpeg", "icon.svg", "icon.webp",
		"logo.png", "logo.jpg", "logo.jpeg", "logo.svg", "logo.webp",
		"app-icon.png", "app-icon.jpg", "app-icon.svg",
	}

	for _, name := range iconFiles {
		path := filepath.Join(m.Cwd, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		const maxIconSize = 2 * 1024 * 1024 // 2MB
		if info.Size() > maxIconSize {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ext := filepath.Ext(path)
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		payload.Icon = &protocol.ProjectIcon{
			Data:     base64.StdEncoding.EncodeToString(data),
			MimeType: mimeType,
		}
		break
	}

	s.sendMessage(protocol.NewSessionMessage(&protocol.ProjectIconResponse{
		Type:    "project_icon_response",
		Payload: payload,
	}))
}
