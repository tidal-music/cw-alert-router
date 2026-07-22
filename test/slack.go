// Copyright 2022 Aspiro AB
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

// SlackServer is a fake Slack API server for testing. It records posted
// messages and uploaded files, and implements the chat.postMessage and
// files.uploadV2 (getUploadURLExternal/completeUploadExternal) flows.
type SlackServer struct {
	Server *httptest.Server

	mu       sync.Mutex
	messages [][]byte
	uploads  map[string][]byte
	fileSeq  int
}

// NewSlackServer starts a fake Slack API server.
func NewSlackServer() *SlackServer {
	s := &SlackServer{uploads: make(map[string][]byte)}
	mux := http.NewServeMux()
	mux.HandleFunc("/chat.postMessage", s.postMessage)
	mux.HandleFunc("/files.getUploadURLExternal", s.getUploadURL)
	mux.HandleFunc("/upload/", s.upload)
	mux.HandleFunc("/files.completeUploadExternal", s.completeUpload)
	s.Server = httptest.NewServer(mux)
	return s
}

// APIURL returns the base URL to pass as the Slack client's alternative API URL.
func (s *SlackServer) APIURL() string {
	return s.Server.URL + "/"
}

// Close shuts the server down.
func (s *SlackServer) Close() {
	s.Server.Close()
}

// Messages returns the raw chat.postMessage request bodies received so far.
func (s *SlackServer) Messages() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte(nil), s.messages...)
}

// Uploads returns the uploaded file contents by file ID.
func (s *SlackServer) Uploads() map[string][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]byte, len(s.uploads))
	for k, v := range s.uploads {
		out[k] = v
	}
	return out
}

func writeJSON(rw http.ResponseWriter, v any) {
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(v)
}

func (s *SlackServer) postMessage(rw http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.messages = append(s.messages, body)
	s.mu.Unlock()

	writeJSON(rw, map[string]any{
		"ok":      true,
		"channel": "XVB123123123",
		"ts":      "123123123123123",
	})
}

func (s *SlackServer) getUploadURL(rw http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.fileSeq++
	fileID := fmt.Sprintf("F%07d", s.fileSeq)
	s.mu.Unlock()

	writeJSON(rw, map[string]any{
		"ok":         true,
		"upload_url": fmt.Sprintf("%s/upload/%s", s.Server.URL, fileID),
		"file_id":    fileID,
	})
}

func (s *SlackServer) upload(rw http.ResponseWriter, r *http.Request) {
	fileID := r.URL.Path[len("/upload/"):]

	var content []byte
	if err := r.ParseMultipartForm(10 << 20); err == nil {
		if f, _, ferr := r.FormFile("file"); ferr == nil {
			content, _ = io.ReadAll(f)
			f.Close()
		}
	}
	s.mu.Lock()
	s.uploads[fileID] = content
	s.mu.Unlock()

	writeJSON(rw, map[string]any{"ok": true})
}

func (s *SlackServer) completeUpload(rw http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	var files []map[string]string
	if err := json.Unmarshal([]byte(r.FormValue("files")), &files); err != nil || len(files) == 0 {
		writeJSON(rw, map[string]any{"ok": false, "error": "invalid_arguments"})
		return
	}
	writeJSON(rw, map[string]any{
		"ok":    true,
		"files": files,
	})
}
