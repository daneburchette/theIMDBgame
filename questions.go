package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

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

func LoadGameFromJSON(state *GameState, filename string) {
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
