package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestMenuFile(t *testing.T, contents []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "menu.json")
	if err := os.WriteFile(path, contents, 0644); err != nil {
		t.Fatalf("failed to write test menu file: %v", err)
	}
	menuFile = path
	return path
}

func resetClients() {
	clientsMu.Lock()
	clients = make(map[int]chan []byte)
	nextClientID = 0
	clientsMu.Unlock()
}

func TestAddMenuItemBroadcasts(t *testing.T) {
	setupTestMenuFile(t, []byte("[]"))
	resetClients()
	ch := make(chan []byte, 1)
	id := addClient(ch)
	defer removeClient(id)

	req := httptest.NewRequest(http.MethodPost, "/api/menu", strings.NewReader(`{"dish":"Cornbread","who":"Jess"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	addMenuItem(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	select {
	case data := <-ch:
		var menu []MenuItem
		if err := json.Unmarshal(data, &menu); err != nil {
			t.Fatalf("failed to decode broadcast data: %v", err)
		}
		if len(menu) != 1 || menu[0].Dish != "Cornbread" {
			t.Fatalf("unexpected broadcast menu: %#v", menu)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not receive broadcast update")
	}

	data, err := os.ReadFile(menuFile)
	if err != nil {
		t.Fatalf("failed to read menu file: %v", err)
	}
	var menu []MenuItem
	if err := json.Unmarshal(data, &menu); err != nil {
		t.Fatalf("failed to decode file menu: %v", err)
	}
	if len(menu) != 1 || menu[0].Dish != "Cornbread" {
		t.Fatalf("menu file does not contain new item: %#v", menu)
	}
}

func TestDeleteMenuItemBroadcasts(t *testing.T) {
	initial := []MenuItem{
		{ID: "1", Dish: "Pumpkin Pie", Who: "Alex"},
		{ID: "2", Dish: "Cranberry Sauce", Who: "Maya"},
	}
	initialData, _ := json.Marshal(initial)
	setupTestMenuFile(t, initialData)
	resetClients()
	ch := make(chan []byte, 1)
	id := addClient(ch)
	defer removeClient(id)

	req := httptest.NewRequest(http.MethodDelete, "/api/menu?id=1", nil)
	rr := httptest.NewRecorder()
	deleteMenuItem(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	select {
	case data := <-ch:
		var menu []MenuItem
		if err := json.Unmarshal(data, &menu); err != nil {
			t.Fatalf("failed to decode broadcast data: %v", err)
		}
		if len(menu) != 1 || menu[0].ID != "2" {
			t.Fatalf("unexpected menu after delete: %#v", menu)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not receive broadcast update after delete")
	}

	fileData, err := os.ReadFile(menuFile)
	if err != nil {
		t.Fatalf("failed to read menu file: %v", err)
	}
	var menu []MenuItem
	if err := json.Unmarshal(fileData, &menu); err != nil {
		t.Fatalf("failed to decode file menu: %v", err)
	}
	if len(menu) != 1 || menu[0].ID != "2" {
		t.Fatalf("menu file still contains deleted item: %#v", menu)
	}
}

type dummyWriter struct {
	headers http.Header
	buffer  bytes.Buffer
}

func (d *dummyWriter) Header() http.Header {
	if d.headers == nil {
		d.headers = make(http.Header)
	}
	return d.headers
}

func (d *dummyWriter) Write(b []byte) (int, error) {
	return d.buffer.Write(b)
}

func (d *dummyWriter) WriteHeader(statusCode int) {}

func (d *dummyWriter) Flush() {}

func TestSendSSEFormatsEvent(t *testing.T) {
	writer := &dummyWriter{}
	flusher := writer
	data := []byte("[{'dish':'Stew'}]\n")
	sendSSE(writer, flusher, data)
	out := writer.buffer.String()
	if !strings.Contains(out, "event: menu") {
		t.Fatalf("expected event line, got %q", out)
	}
	if !strings.Contains(out, `data: [{'dish':'Stew'}]`) {
		t.Fatalf("expected data payload, got %q", out)
	}
}
