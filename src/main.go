package main

import (
	"fmt"
	"log"
	"net/http"

	"friendsgiving/src/app"
)

func main() {
	menuApp := app.New("/app/data/menu.json")
	http.HandleFunc("/api/menu", menuApp.HandleMenu)
	http.HandleFunc("/api/menu/stream", menuApp.StreamMenu)
	http.Handle("/", http.FileServer(http.Dir("/app/static")))

	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
