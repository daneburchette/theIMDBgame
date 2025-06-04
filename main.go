package main

import (
	"bufio"
	"encoding/json"
	"flag"
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
	Name     string
	Score    int
	Guess    float64
	Choice   string
	Answered bool
	Active   bool
}

type Question struct {
	Number       int
	Title        string
	Year         int
	Cast         []string
	Desc         string
	UserCount    int
	Rating       float64
	RoundNumber  int
	Points       int
	ActivePlayer int
	FinalRound   bool
}

func (q *Question) PrintQuestion() {
	fmt.Printf("\nMovie: %s (%d)\n", q.Title, q.Year)
	fmt.Printf("Cast: %v\n", q.Cast)
	fmt.Printf("Description:\n\t%s\n", q.Desc)
	fmt.Printf("\nScore: %0.1f as voted by %d Users\n\n", q.Rating, q.UserCount)
}

type GameState struct {
	Players             []Player
	PlayerCount         int
	ExpectedPlayerCount int
	GameName            string     `json:"GameName"`
	Questions           []Question `json:"Questions"`
	QuestionNumber      int
	CurrentQuestion     Question
	PointValue          int
	PlayerInputs        int
	RoundAdvanced       bool
	LoggedIn            bool
	Mutex               sync.Mutex
}

type PlayerCount struct {
	Number       int
	RoundNumber  int
	Points       int
	ActivePlayer int
	FinalRound   bool
}

type PlayerCountSet struct {
	ParsedJson []PlayerCount `json:"Questions"`
}

func CreateGameState(state *GameState, filename string, playerCount *int) {
	loadGameFromJSON(state, filename)
	state.QuestionNumber = -1
	state.ExpectedPlayerCount = *playerCount
}

func (g *GameState) NextQuestion() {
	g.Mutex.Lock()
	defer g.Mutex.Unlock()

	if g.QuestionNumber < 0 {
		log.Println("Game Begin")
	}
	g.QuestionNumber++
	g.CurrentQuestion = g.Questions[g.QuestionNumber]
	g.PlayerInputs = 0
	g.RoundAdvanced = true

	for i := range g.Players {
		g.Players[i].Guess = 0
		g.Players[i].Choice = ""
		g.Players[i].Answered = false
	}
	log.Println("Advanced to next round")
	g.CurrentQuestion.PrintQuestion()
}

func (g *GameState) ScoreQuestion() {
	var target float64
	for _, player := range g.Players {
		if player.Active {
			target = player.Guess
		}
	}
	var activeScore int
	var exactScore bool
	var exactStole bool
	switch {
	case g.CurrentQuestion.Rating > target:
		for i := range g.Players {
			if g.Players[i].Choice == "higher" && !g.Players[i].Active {
				g.Players[i].Score += g.PointValue
				log.Printf("%s scored %d points\n", g.Players[i].Name, g.PointValue)
			} else {
				activeScore += g.PointValue
			}
		}
	case g.CurrentQuestion.Rating < target:
		for i := range g.Players {
			if g.Players[i].Choice == "lower" && !g.Players[i].Active {
				log.Printf("%s scored %d points\n", g.Players[i].Name, g.PointValue)
				g.Players[i].Score += g.PointValue
			} else {
				activeScore += g.PointValue
			}
		}
	case g.CurrentQuestion.Rating == target:
		exactScore = true
		for i := range g.Players {
			if g.Players[i].Choice == "exact" && !g.Players[i].Active {
				g.Players[i].Score += g.PointValue + 5
				log.Printf("%s scored %d points AND stole the 5 point bonus!\n", g.Players[i].Name, g.PointValue)
				exactStole = true
			} else {
				activeScore += g.PointValue
			}
		}
	}
	for i := range g.Players {
		if g.Players[i].Active {
			g.Players[i].Score += activeScore
			log.Printf("%s scored %d points\n", g.Players[i].Name, activeScore)
			if exactScore && !exactStole {
				g.Players[i].Score += 5
				log.Printf("%s scored a cool 5 point bonus!\n", g.Players[i].Name)
			} else if exactScore && exactStole {
				log.Printf("%s lost their cool 5 point bonus!\n", g.Players[i].Name)
			}
		}
	}
}

var state GameState

func main() {
	port := flag.Int("port", 8080, "Port number for the server")
	playerCount := flag.Int("players", 1, "Number of Players for game")
	flag.Parse()

	CreateGameState(&state, "static/JSON/questions/questions.json", playerCount)

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

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	err := tmpl.Execute(w, &state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func gameHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Game Handler Triggered")
	state.Mutex.Lock()
	defer state.Mutex.Unlock()

	tmpl := template.Must(template.ParseFiles("templates/game.html"))
	err := tmpl.Execute(w, &state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func loadGameFromJSON(state *GameState, filename string) {
	var pcFilename string
	switch state.ExpectedPlayerCount {
	case 1:
		pcFilename = "static/JSON/playercounts/1player.json"
	case 2:
		pcFilename = "static/JSON/playercounts/2player.json"
	case 3:
		pcFilename = "static/JSON/playercounts/3player.json"
	case 4:
		pcFilename = "static/JSON/playercounts/4player.json"
	case 5:
		pcFilename = "static/JSON/playercounts/5player.json"
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read game data from JSON: %v", err)
	}
	err = json.Unmarshal(data, state)
	if err != nil {
		log.Fatalf("Failed to read game data from JSON: %v", err)
	}
	var rounds PlayerCountSet
	pcdata, err := os.ReadFile(pcFilename)
	if err != nil {
		log.Fatalf("Failed to read game data from JSON: %v", err)
	}
	err = json.Unmarshal(pcdata, &rounds)
	if err != nil {
		log.Fatalf("Failed to read game data from JSON: %v", err)
	}
	questionUpdate(state, &rounds)
}

func questionUpdate(state *GameState, rounds *PlayerCountSet) {
	for i := range rounds.ParsedJson {
		if rounds.ParsedJson[i].FinalRound && rounds.ParsedJson[i].RoundNumber != len(state.Questions) {
			finalNumber := rounds.ParsedJson[i].RoundNumber
			state.Questions = append(state.Questions[:finalNumber], state.Questions[len(state.Questions)-1:]...)
			state.Questions[finalNumber].Number = finalNumber
		}
		state.Questions[i].RoundNumber = rounds.ParsedJson[i].RoundNumber
		state.Questions[i].Points = rounds.ParsedJson[i].Points
		state.Questions[i].ActivePlayer = rounds.ParsedJson[i].ActivePlayer
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
	defer state.Mutex.Unlock()
	state.Players = append(state.Players, Player{Name: name})
	state.PlayerCount++
	log.Printf("%s has joined. Count: %d/%d\n", name, state.PlayerCount, state.ExpectedPlayerCount)

	if state.ExpectedPlayerCount == state.PlayerCount {
		state.LoggedIn = true
		log.Println("All players joined, starting game.")
		state.Mutex.Unlock()
		state.NextQuestion()
		state.Mutex.Lock()
	}

	log.Println("Redirecting to /game")
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
	defer state.Mutex.Unlock()

	for i := range state.Players {
		if state.Players[i].Name == name && !state.Players[i].Answered {
			state.Players[i].Guess = guess
			state.Players[i].Choice = choice
			state.Players[i].Answered = true
			log.Printf("%s submitted a guess of %.1f\n", name, guess)
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

	// w.WriteHeader(http.StatusOK)

	http.Redirect(w, r, "/game?submitted=true", http.StatusSeeOther)
}

func nextQuestionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state.Mutex.Lock()
	defer state.Mutex.Unlock()

	if state.QuestionNumber+1 < len(state.Questions) && !state.RoundAdvanced {
		state.Mutex.Unlock()
		state.NextQuestion()
		state.Mutex.Lock()
	} else if state.QuestionNumber+1 >= len(state.Questions) {
		// eventual results page redirect
		log.Println("End of Game")
	}

	http.Redirect(w, r, "/game", http.StatusSeeOther)
}
