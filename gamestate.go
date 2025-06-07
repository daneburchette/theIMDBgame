package main

import (
	"log"
	"sync"
)

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

func CreateGameState(state *GameState, filename string, playerCount int) {
	state.ExpectedPlayerCount = playerCount
	state.QuestionNumber = -1

	LoadGameFromJSON(state, filename)
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

type Player struct {
	Name     string
	Score    int
	Guess    float64
	Choice   string
	Answered bool
	Active   bool
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
