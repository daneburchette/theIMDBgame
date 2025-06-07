package main

import (
	"bufio"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	err := tmpl.Execute(w, &state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
func gameHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Game Handler Triggered")
	// lock mutex
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	// create question page
	tmpl := template.Must(template.ParseFiles("templates/game.html"))
	err := tmpl.Execute(w, &state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
