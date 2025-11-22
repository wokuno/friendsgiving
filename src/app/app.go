package app

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MenuItem captures who is bringing which dish.
type MenuItem struct {
	ID   string `json:"id"`
	Dish string `json:"dish"`
	Who  string `json:"who"`
}

var defaultMenu = []MenuItem{
	{ID: "1763786780838787402", Dish: "Turkey", Who: "Will"},
	{ID: "1763786910210202650", Dish: "Dessert", Who: "Sarah"},
}

// App hosts the state for the menu service.
type App struct {
	menuFile     string
	mu           sync.Mutex
	clients      map[int]chan []byte
	clientsMu    sync.Mutex
	nextClientID int
}

// New creates a menu application backed by the provided file path.
func New(menuFile string) *App {
	app := &App{
		menuFile: menuFile,
		clients:  make(map[int]chan []byte),
	}
	app.ensureMenuFile()
	return app
}

// HandleMenu exposes the REST CRUD operations for the menu.
func (a *App) HandleMenu(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleGetMenu(w)
	case http.MethodPost:
		a.handleAddMenuItem(w, r)
	case http.MethodDelete:
		a.handleDeleteMenuItem(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// StreamMenu publishes menu updates to SSE listeners.
func (a *App) StreamMenu(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}

	ch := make(chan []byte, 5)
	id := a.addClient(ch)
	defer a.removeClient(id)

	menu, err := a.readMenu()
	if err == nil {
		data, err := json.Marshal(menu)
		if err == nil {
			a.sendSSE(w, flusher, data)
		}
	}

	for {
		select {
		case data := <-ch:
			a.sendSSE(w, flusher, data)
		case <-r.Context().Done():
			return
		}
	}
}

// ObserveMenuUpdates returns a buffered channel that receives menu payloads.
func (a *App) ObserveMenuUpdates() (<-chan []byte, func()) {
	ch := make(chan []byte, 5)
	id := a.addClient(ch)
	return ch, func() { a.removeClient(id) }
}

func (a *App) handleGetMenu(w http.ResponseWriter) {
	menu, err := a.readMenu()
	if err != nil {
		http.Error(w, "Failed to read menu", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(menu); err != nil {
		http.Error(w, "Failed to encode menu", http.StatusInternalServerError)
	}
}

func (a *App) handleAddMenuItem(w http.ResponseWriter, r *http.Request) {
	var newItem MenuItem
	if err := json.NewDecoder(r.Body).Decode(&newItem); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if newItem.Dish == "" || newItem.Who == "" {
		http.Error(w, "Dish and Who are required", http.StatusBadRequest)
		return
	}

	newItem.ID = strconv.FormatInt(time.Now().UnixNano(), 10)
	menu, err := a.addMenuItem(newItem)
	if err != nil {
		http.Error(w, "Failed to save menu", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(menu)
}

func (a *App) handleDeleteMenuItem(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	_, err := a.deleteMenuItem(id)
	if err != nil {
		http.Error(w, "Failed to save menu", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *App) addMenuItem(newItem MenuItem) ([]MenuItem, error) {
	menu, err := a.readMenu()
	if err != nil {
		return nil, err
	}

	menu = append(menu, newItem)
	data, err := a.writeMenu(menu)
	if err != nil {
		return nil, err
	}

	a.broadcastMenuUpdate(data)
	return menu, nil
}

func (a *App) deleteMenuItem(id string) ([]MenuItem, error) {
	menu, err := a.readMenu()
	if err != nil {
		return nil, err
	}

	var newMenu []MenuItem
	for _, item := range menu {
		if item.ID != id {
			newMenu = append(newMenu, item)
		}
	}

	data, err := a.writeMenu(newMenu)
	if err != nil {
		return nil, err
	}

	a.broadcastMenuUpdate(data)
	return newMenu, nil
}

func (a *App) readMenu() ([]MenuItem, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := os.ReadFile(a.menuFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []MenuItem{}, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return []MenuItem{}, nil
	}

	var menu []MenuItem
	if err := json.Unmarshal(data, &menu); err != nil {
		return nil, err
	}
	return menu, nil
}

func (a *App) writeMenu(menu []MenuItem) ([]byte, error) {
	data, err := json.MarshalIndent(menu, "", "    ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(a.menuFile, data, 0644); err != nil {
		return nil, err
	}

	return data, nil
}

func (a *App) ensureMenuFile() {
	if _, err := os.Stat(a.menuFile); os.IsNotExist(err) {
		data, err := json.MarshalIndent(defaultMenu, "", "    ")
		if err != nil {
			log.Printf("Failed to marshal default menu: %v", err)
			return
		}
		if err := os.WriteFile(a.menuFile, data, 0644); err != nil {
			log.Printf("Failed to create default menu file: %v", err)
		}
	}
}

func (a *App) sendSSE(w http.ResponseWriter, flusher http.Flusher, data []byte) {
	fmt.Fprintf(w, "event: menu\n")
	for _, line := range strings.Split(string(data), "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
	flusher.Flush()
}

func (a *App) addClient(ch chan []byte) int {
	a.clientsMu.Lock()
	defer a.clientsMu.Unlock()

	a.nextClientID++
	a.clients[a.nextClientID] = ch
	return a.nextClientID
}

func (a *App) removeClient(id int) {
	a.clientsMu.Lock()
	defer a.clientsMu.Unlock()
	if ch, ok := a.clients[id]; ok {
		close(ch)
		delete(a.clients, id)
	}
}

func (a *App) broadcastMenuUpdate(data []byte) {
	a.clientsMu.Lock()
	defer a.clientsMu.Unlock()
	for _, ch := range a.clients {
		select {
		case ch <- data:
		default:
		}
	}
}
