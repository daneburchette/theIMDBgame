package main

import (
	"flag"
	"fmt"
	"net/http"
)

const filepath string = "static/JSON/"

var state GameState

func main() {
	port := flag.Int("port", 8080, "Port number for the server")
	playerCount := flag.Int("players", 1, "Number of Players for game")
	json := flag.String("json", "questions.json", "Path to json game file")
	flag.Parse()

	gamePath := fmt.Sprintf("%squestions/%s", filepath, *json)

	CreateGameState(&state, gamePath, *playerCount)

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/game", gameHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/next", nextQuestionHandler)
	http.HandleFunc("/submit", submitHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("Server running at http://localhost%s\n", addr)
	http.ListenAndServe(addr, nil)
}
