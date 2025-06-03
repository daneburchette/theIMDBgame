package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
)

type Player struct {
	Name   string
	Score  int
	Guess  float64
	Choice string
	Active bool
}

type Question struct {
	Number    int
	Title     string
	Year      int
	Cast      []string
	Desc      string
	UserCount int
	Rating    float64
	Final     bool
}

type GameState struct {
	Players         []Player
	PlayerCount     int
	GameName        string     `json:"GameName"`
	Questions       []Question `json:"Questions"`
	QuestionNumber  int
	CurrentQuestion Question
	PlayerInputs    int
	Mutex           sync.Mutex
	RoundAdvanced   bool
}

func CreateGameState(state *GameState, filename string) {
	loadGameFromJSON(state, filename)
	state.CurrentQuestion = state.Questions[0]
	state.RoundAdvanced = true
}

func (g *GameState) NextQuestion() {
	g.QuestionNumber++
	g.CurrentQuestion = g.Questions[g.QuestionNumber]
	g.PlayerInputs = 0
	g.RoundAdvanced = true
	log.Println("Advanced to next round")
}

var state GameState

func main() {
	CreateGameState(&state, "static/JSON/Questions/Questions.json")

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/game", gameHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/next", nextRoundHandler)
	http.HandleFunc("/submit", submitHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("Server running at http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	err := tmpl.Execute(w, &state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func gameHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/game.html"))
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	err := tmpl.Execute(w, &state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func loadGameFromJSON(state *GameState, filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read game data from JSON: %v", err)
	}
	err = json.Unmarshal(data, state)
	if err != nil {
		log.Fatalf("Failed to read game data from JSON: %v", err)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:  "playerID",
		Value: url.QueryEscape(name),
		Path:  "/",
	})
	state.Mutex.Lock()
	state.Players = append(state.Players, Player{Name: name})
	state.PlayerCount++
	defer state.Mutex.Unlock()
	log.Printf("%s has joined the game", name)
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	// name := r.URL.Query().Get("name")
	// name := r.FormValue("name")
	cookie, err := r.Cookie("playerID")
	if err != nil {
		http.Error(w, "Player not identified", http.StatusUnauthorized)
		return
	}
	name, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		http.Error(w, "Invalid player ID", http.StatusBadRequest)
		return
	}
	guessStr := r.FormValue("guess")
	choice := r.FormValue("choice")
	guess, _ := strconv.ParseFloat(guessStr, 64)

	state.Mutex.Lock()
	for i := range state.Players {
		if state.Players[i].Name == name {
			state.Players[i].Guess = guess
			log.Printf("%s submitted a guess of %.1f\n", name, guess)
			state.Players[i].Choice = choice
			log.Printf("%s submitted a choice of %s\n", name, choice)
			state.PlayerInputs++
			break
		}
	}
	if state.PlayerInputs == len(state.Players) {
		log.Println("All guesses submitted. Press ENTER to score the round...")
		reader := bufio.NewReader(os.Stdin)
		reader.ReadString('\n')
		// scoreRound()
		state.RoundAdvanced = false
		log.Println("Round Scored.")
	}

	state.Mutex.Unlock()
	// w.WriteHeader(http.StatusOK)

	http.Redirect(w, r, "/game?submitted=true", http.StatusSeeOther)
}

func nextRoundHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state.Mutex.Lock()
	defer state.Mutex.Unlock()

	if state.QuestionNumber < len(state.Questions) && !state.RoundAdvanced {
		state.NextQuestion()
	} else if state.QuestionNumber == len(state.Questions) {
		// eventual results page redirect
		log.Println("End of Game")
	}
	fmt.Printf("%+v", state.CurrentQuestion)

	http.Redirect(w, r, "/game", http.StatusSeeOther)
}
