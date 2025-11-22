package app_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"friendsgiving/src/app"
)

func setupTestMenuFile(t *testing.T, contents []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "menu.json")
	if err := os.WriteFile(path, contents, 0644); err != nil {
		t.Fatalf("failed to write test menu file: %v", err)
	}
	return path
}

func TestNewCreatesDefaultFile(t *testing.T) {
	// Start with a non-existent file and ensure New writes the default menu.
	dir := t.TempDir()
	menuPath := filepath.Join(dir, "menu.json")

	if _, err := os.Stat(menuPath); !os.IsNotExist(err) {
		t.Fatalf("expected file to not exist yet")
	}

	_ = app.New(menuPath)

	data, err := os.ReadFile(menuPath)
	if err != nil {
		t.Fatalf("expected menu file to be created, got error: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected default menu to be written, file is empty")
	}

	var menu []app.MenuItem
	if err := json.Unmarshal(data, &menu); err != nil {
		t.Fatalf("failed to decode default menu: %v", err)
	}
	if len(menu) == 0 {
		t.Fatalf("expected non-empty default menu")
	}
}

func TestHandleGetMenu(t *testing.T) {
	initial := []app.MenuItem{{ID: "1", Dish: "Stuffing", Who: "Pat"}}
	initialData, _ := json.Marshal(initial)
	menuPath := setupTestMenuFile(t, initialData)
	server := app.New(menuPath)

	req := httptest.NewRequest(http.MethodGet, "/api/menu", nil)
	rr := httptest.NewRecorder()
	server.HandleMenu(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json content type, got %q", ct)
	}

	var got []app.MenuItem
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 1 || got[0].Dish != "Stuffing" {
		t.Fatalf("unexpected menu from GET: %#v", got)
	}
}

func TestHandleMenuMethodNotAllowed(t *testing.T) {
	menuPath := setupTestMenuFile(t, []byte("[]"))
	server := app.New(menuPath)

	req := httptest.NewRequest(http.MethodPut, "/api/menu", nil)
	rr := httptest.NewRecorder()
	server.HandleMenu(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleAddMenuItemValidation(t *testing.T) {
	menuPath := setupTestMenuFile(t, []byte("[]"))
	server := app.New(menuPath)

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/menu", strings.NewReader("{"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		server.HandleMenu(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/menu", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		server.HandleMenu(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})
}

func TestHandleDeleteMenuItemValidation(t *testing.T) {
	menuPath := setupTestMenuFile(t, []byte("[]"))
	server := app.New(menuPath)

	req := httptest.NewRequest(http.MethodDelete, "/api/menu", nil) // no id
	rr := httptest.NewRecorder()
	server.HandleMenu(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing id, got %d", rr.Code)
	}
}

func TestAddMenuItemBroadcasts(t *testing.T) {
	menuPath := setupTestMenuFile(t, []byte("[]"))
	server := app.New(menuPath)
	updates, cleanup := server.ObserveMenuUpdates()
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/menu", strings.NewReader("{\"dish\":\"Cornbread\",\"who\":\"Jess\"}"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.HandleMenu(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	select {
	case data := <-updates:
		var menu []app.MenuItem
		if err := json.Unmarshal(data, &menu); err != nil {
			t.Fatalf("failed to decode broadcast data: %v", err)
		}
		if len(menu) != 1 || menu[0].Dish != "Cornbread" {
			t.Fatalf("unexpected broadcast menu: %#v", menu)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not receive broadcast update")
	}

	data, err := os.ReadFile(menuPath)
	if err != nil {
		t.Fatalf("failed to read menu file: %v", err)
	}
	var menu []app.MenuItem
	if err := json.Unmarshal(data, &menu); err != nil {
		t.Fatalf("failed to decode file menu: %v", err)
	}
	if len(menu) != 1 || menu[0].Dish != "Cornbread" {
		t.Fatalf("menu file does not contain new item: %#v", menu)
	}
}

func TestDeleteMenuItemBroadcasts(t *testing.T) {
	initial := []app.MenuItem{
		{ID: "1", Dish: "Pumpkin Pie", Who: "Alex"},
		{ID: "2", Dish: "Cranberry Sauce", Who: "Maya"},
	}
	initialData, _ := json.Marshal(initial)
	menuPath := setupTestMenuFile(t, initialData)
	server := app.New(menuPath)
	updates, cleanup := server.ObserveMenuUpdates()
	defer cleanup()

	req := httptest.NewRequest(http.MethodDelete, "/api/menu?id=1", nil)
	rr := httptest.NewRecorder()
	server.HandleMenu(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	select {
	case data := <-updates:
		var menu []app.MenuItem
		if err := json.Unmarshal(data, &menu); err != nil {
			t.Fatalf("failed to decode broadcast data: %v", err)
		}
		if len(menu) != 1 || menu[0].ID != "2" {
			t.Fatalf("unexpected menu after delete: %#v", menu)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("did not receive broadcast update after delete")
	}

	data, err := os.ReadFile(menuPath)
	if err != nil {
		t.Fatalf("failed to read menu file: %v", err)
	}
	var menu []app.MenuItem
	if err := json.Unmarshal(data, &menu); err != nil {
		t.Fatalf("failed to decode file menu: %v", err)
	}
	if len(menu) != 1 || menu[0].ID != "2" {
		t.Fatalf("menu file still contains deleted item: %#v", menu)
	}
}

func TestReadMenuHandlesMissingFile(t *testing.T) {
	dir := t.TempDir()
	menuPath := filepath.Join(dir, "menu.json")
	server := app.New(menuPath)

	// Delete the file so readMenu has to handle os.IsNotExist.
	if err := os.Remove(menuPath); err != nil {
		t.Fatalf("failed to remove menu file: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/menu", nil)
	rr := httptest.NewRecorder()
	server.HandleMenu(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 when menu file is missing, got %d", rr.Code)
	}

	var menu []app.MenuItem
	if err := json.NewDecoder(rr.Body).Decode(&menu); err != nil {
		t.Fatalf("failed to decode menu: %v", err)
	}
	if len(menu) != 0 {
		t.Fatalf("expected empty menu when file missing, got %#v", menu)
	}
}

func TestStreamMenuSendsInitialAndUpdates(t *testing.T) {
	initial := []app.MenuItem{{ID: "1", Dish: "Mashed Potatoes", Who: "Jamie"}}
	initialData, _ := json.Marshal(initial)
	menuPath := setupTestMenuFile(t, initialData)
	server := app.New(menuPath)

	req := httptest.NewRequest(http.MethodGet, "/api/menu/stream", nil)
	rr := httptest.NewRecorder()

	// Wrap ResponseRecorder with a flusher-compatible ResponseWriter.
	flushingWriter := &flushRecorder{ResponseRecorder: rr}

	done := make(chan struct{})
	go func() {
		server.StreamMenu(flushingWriter, req)
		close(done)
	}()

	// Give StreamMenu a little time to write the initial event.
	time.Sleep(50 * time.Millisecond)

	// We only assert on the initial push here; exercising live updates is
	// already covered by TestAddMenuItemBroadcasts and TestDeleteMenuItemBroadcasts.
	body := rr.Body.String()
	if !strings.Contains(body, "event: menu") {
		t.Fatalf("expected SSE event header in stream, got %q", body)
	}
	if !strings.Contains(body, "Mashed Potatoes") {
		t.Fatalf("expected stream to contain initial menu, got %q", body)
	}

	// Avoid leaking the goroutine in case it hasn't exited yet.
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
	}
}

// flushRecorder adapts httptest.ResponseRecorder to implement http.Flusher.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {
	// ResponseRecorder writes to an in-memory buffer, so Flush is a no-op.
}

// Ensure flushRecorder also satisfies io.Writer so it's safe for fmt.Fprint.
var _ http.Flusher = (*flushRecorder)(nil)
var _ io.Writer = (*flushRecorder)(nil)
