package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type MenuItem struct {
	ID   string `json:"id"`
	Dish string `json:"dish"`
	Who  string `json:"who"`
}

var (
	menuFile = "menu.json"
	mu       sync.Mutex
)

func main() {
	ensureMenuFile()
	http.HandleFunc("/api/menu", handleMenu)
	http.Handle("/", http.FileServer(http.Dir(".")))

	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleMenu(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getMenu(w, r)
	case http.MethodPost:
		addMenuItem(w, r)
	case http.MethodDelete:
		deleteMenuItem(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getMenu(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(menuFile)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}
		http.Error(w, "Failed to read menu", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func addMenuItem(w http.ResponseWriter, r *http.Request) {
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

	mu.Lock()
	defer mu.Unlock()

	// Read existing menu
	var menu []MenuItem
	data, err := os.ReadFile(menuFile)
	if err == nil {
		json.Unmarshal(data, &menu)
	}

	// Append new item
	menu = append(menu, newItem)

	// Write back to file
	updatedData, err := json.MarshalIndent(menu, "", "    ")
	if err != nil {
		http.Error(w, "Failed to encode menu", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(menuFile, updatedData, 0644); err != nil {
		http.Error(w, "Failed to save menu", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(updatedData)
}

func deleteMenuItem(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	var menu []MenuItem
	data, err := os.ReadFile(menuFile)
	if err == nil {
		json.Unmarshal(data, &menu)
	}

	var newMenu []MenuItem
	for _, item := range menu {
		if item.ID != id {
			newMenu = append(newMenu, item)
		}
	}

	updatedData, err := json.MarshalIndent(newMenu, "", "    ")
	if err != nil {
		http.Error(w, "Failed to encode menu", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(menuFile, updatedData, 0644); err != nil {
		http.Error(w, "Failed to save menu", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ensureMenuFile() {
	if _, err := os.Stat(menuFile); os.IsNotExist(err) {
		defaultMenu := []MenuItem{
			{
				ID:   "1763786780838787402",
				Dish: "Turkey",
				Who:  "Will",
			},
			{
				ID:   "1763786910210202650",
				Dish: "Dessert",
				Who:  "Sarah",
			},
		}

		data, err := json.MarshalIndent(defaultMenu, "", "    ")
		if err != nil {
			log.Printf("Failed to marshal default menu: %v", err)
			return
		}

		if err := os.WriteFile(menuFile, data, 0644); err != nil {
			log.Printf("Failed to create default menu file: %v", err)
		} else {
			fmt.Println("Created default menu.json")
		}
	}
}
