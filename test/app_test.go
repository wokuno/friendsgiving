package app_test

import (
	"encoding/json"
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
