package imdbweb

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
)

// Player represents a game participant.
type Player struct {
	Name   string
	Score  int
	Guess  float64
	Choice string // For higher/lower or final round selection
}

// RoundType represents the type of a round (normal or final).
type RoundType int

const (
	// Normal round with an active player and higher/lower guesses
	Normal RoundType = iota
	// Final round where all players guess independently
	Final
)

// Round stores data about a single round of the game.
type Round struct {
	Number        int
	MovieTitle    string
	MovieYear     int
	MovieCast     []string
	MovieDesc     string
	ActualRating  float64
	ActivePlayer  int
	Type          RoundType
	GuessesIn     int
	RatingGuessed bool
}

// GameState holds the current state of the game.
type GameState struct {
	Players      []Player
	CurrentRound Round
	RoundHistory []Round
	AllRounds    []Round
	Mutex        sync.Mutex
}

// Global game state
var state = GameState{
	Players: []Player{
		{Name: "Alice"},
		{Name: "Bob"},
		{Name: "Carol"},
	},
}

// main initializes the game, loads rounds, and starts the web server.
func main() {
	loadRoundsFromJSON("static/rounds.json")
	state.CurrentRound = state.AllRounds[0]

	http.HandleFunc("/", gamePageHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/next", nextRoundHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("Server running at http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

// loadRoundsFromJSON reads and unmarshals round data from a JSON file.
func loadRoundsFromJSON(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read rounds JSON: %v", err)
	}
	var rounds []Round
	err = json.Unmarshal(data, &rounds)
	if err != nil {
		log.Fatalf("Failed to parse rounds JSON: %v", err)
	}
	state.AllRounds = rounds
}

// gamePageHandler renders the game page using HTML templates.
func gamePageHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/game.html"))
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	tmpl.Execute(w, &state)
}

// submitHandler records a player's guess and prompts CLI confirmation to score the round.
func submitHandler(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	guessStr := r.FormValue("guess")
	choice := r.FormValue("choice")
	guess, _ := strconv.ParseFloat(guessStr, 64)

	state.Mutex.Lock()
	for i := range state.Players {
		if state.Players[i].Name == name {
			state.Players[i].Guess = guess
			state.Players[i].Choice = choice
			state.CurrentRound.GuessesIn++
			log.Printf("%s submitted a guess: %.1f, choice: %s", name, guess, choice)
			break
		}
	}
	if state.CurrentRound.GuessesIn == 3 {
		log.Println("All guesses submitted. Press ENTER to score the round...")
		reader := bufio.NewReader(os.Stdin)
		reader.ReadString('\n')
		scoreRound()
		log.Println("Round scored.")
	}
	state.Mutex.Unlock()
	w.WriteHeader(http.StatusOK)
}

// scoreRound applies game logic to update scores based on submitted guesses.
func scoreRound() {
	r := &state.CurrentRound
	players := state.Players

	if r.Type == Normal {
		ap := r.ActivePlayer
		apGuess := players[ap].Guess
		for i := range players {
			if i == ap {
				if math.Abs(players[i].Guess-r.ActualRating) < 0.05 {
					players[i].Score += 3 * getMultiplier(r.Number)
				}
				continue
			}
			correct := (r.ActualRating > apGuess && players[i].Choice == "higher") || (r.ActualRating < apGuess && players[i].Choice == "lower")
			if correct {
				players[i].Score += 1 * getMultiplier(r.Number)
			} else {
				players[ap].Score += 1 * getMultiplier(r.Number)
			}
		}
	} else {
		type guessResult struct {
			Index int
			Diff  float64
			Over  bool
		}
		var results []guessResult
		for i := range players {
			diff := math.Abs(players[i].Guess - r.ActualRating)
			over := players[i].Guess > r.ActualRating
			results = append(results, guessResult{i, diff, over})
		}
		sort.Slice(results, func(i, j int) bool {
			if results[i].Diff == results[j].Diff {
				return !results[i].Over && results[j].Over
			}
			return results[i].Diff < results[j].Diff
		})
		winner := results[0]
		players[winner.Index].Score += 100 // Arbitrarily high value
		log.Printf("Final round winner: %s\n", players[winner.Index].Name)
	}
	state.RoundHistory = append(state.RoundHistory, *r)
}

// getMultiplier returns the score multiplier based on round number.
func getMultiplier(round int) int {
	switch {
	case round <= 3:
		return 1
	case round <= 6:
		return 2
	case round <= 9:
		return 3
	default:
		return 1
	}
}

// statusHandler returns the current game state as JSON.
func statusHandler(w http.ResponseWriter, r *http.Request) {
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	json.NewEncoder(w).Encode(&state)
}

// nextRoundHandler advances the game to the next round and resets state.
func nextRoundHandler(w http.ResponseWriter, r *http.Request) {
	state.Mutex.Lock()
	nextIndex := state.CurrentRound.Number
	if nextIndex >= len(state.AllRounds) {
		log.Println("No more rounds available.")
		state.Mutex.Unlock()
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	state.CurrentRound = state.AllRounds[nextIndex]
	for i := range state.Players {
		state.Players[i].Guess = 0
		state.Players[i].Choice = ""
	}
	state.CurrentRound.GuessesIn = 0
	log.Printf("Started Round %d", state.CurrentRound.Number)
	state.Mutex.Unlock()
	w.WriteHeader(http.StatusOK)
}
