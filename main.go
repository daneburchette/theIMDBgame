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

func CreateGameState(state *GameState, filename string, playerCount int) {
	state.ExpectedPlayerCount = playerCount
	state.QuestionNumber = -1

	loadGameFromJSON(state, filename)
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
	log.Println("Scoring Triggered")
	var target float64
	for _, player := range g.Players {
		if player.Active {
			target = player.Guess
		}
	}
	var activeScore int
	var targetChoice string
	var exactScore bool
	var exactStole bool
	switch {
	case g.CurrentQuestion.Rating > target:
		targetChoice = "higher"
	case g.CurrentQuestion.Rating < target:
		targetChoice = "lower"
	case g.CurrentQuestion.Rating == target:
		targetChoice = "exact"
		exactScore = true
	}
	for i := range g.Players {
		if g.Players[i].Choice == targetChoice && !g.Players[i].Active {
			log.Printf("%s scored %d points\n", g.Players[i].Name, g.PointValue)
			g.Players[i].Score += g.PointValue
			if exactScore {
				log.Printf("AND %s stole the cool 5 point bonus!\n", g.Players[i].Name)
				g.Players[i].Score += 5
				exactStole = true
			}
		} else {
			activeScore += g.PointValue
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

func (g *GameState) SoloScoreQuestion() {
	var exact bool
	difference := g.CurrentQuestion.Rating - g.Players[0].Guess
	switch {
	case difference < 0:
		difference *= -1
	case difference == 0:
		exact = true
	default:
		// pass
	}
	if difference < 1.1 {
		g.Players[0].Score += g.CurrentQuestion.Points
		log.Printf("%s scored %d points\n", g.Players[0].Name, g.CurrentQuestion.Points)
		if exact {
			g.Players[0].Score += 5
			log.Printf("%s scored a cool 5 point bonus!\n", g.Players[0].Name)
		}
	}
}

func (g *GameState) setActivePlayer() {
	activePlayer := g.CurrentQuestion.ActivePlayer
	for i := range g.Players {
		if g.Players[i].Active {
			g.Players[i].Active = false
		}
	}
	g.Players[activePlayer].Active = true
}

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
	PCountPath := fmt.Sprintf("%splayercounts/%dplayer.json", filepath, state.ExpectedPlayerCount)
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read game data from JSON: %v", err)
	}
	err = json.Unmarshal(data, state)
	if err != nil {
		log.Fatalf("Failed to read game data from JSON: %v", err)
	}
	var rounds PlayerCountSet
	pcdata, err := os.ReadFile(PCountPath)
	log.Println(PCountPath)
	if err != nil {
		log.Fatalf("Failed to read playercount data from JSON: %v", err)
	}
	err = json.Unmarshal(pcdata, &rounds)
	if err != nil {
		log.Fatalf("Failed to unmarshal playercount data from JSON: %v", err)
	}
	questionUpdate(state, &rounds)
}

func questionUpdate(state *GameState, rounds *PlayerCountSet) {
	for i := range rounds.ParsedJson {
		if rounds.ParsedJson[i].FinalRound && rounds.ParsedJson[i].Number != len(state.Questions) {
			finalNumber := rounds.ParsedJson[i].Number
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

	var firstActive bool
	if state.PlayerCount == 0 {
		firstActive = true
	}
	state.Players = append(state.Players, Player{Name: name, Active: firstActive})
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
		state.RoundAdvanced = false
		state.ScoreQuestion()
		state.setActivePlayer()
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
