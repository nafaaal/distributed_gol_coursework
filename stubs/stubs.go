package stubs

import "uk.ac.bris.cs/gameoflife/util"

var TurnHandler = "GameOfLifeOperation.CompleteTurn"
var AliveCellGetter = "GameOfLifeOperation.GetAliveCell"
var Shutdown = "GameOfLifeOperation.Shutdown"
var Reset = "GameOfLifeOperation.ResetState"
var PauseAndResume = "GameOfLifeOperation.PauseAndResume"
var GetWorldPerTurn = "GameOfLifeOperation.GetWorldPerTurn"
var ProcessSlice = "Node.ProcessSlice"
var GetWorld = "GameOfLifeOperation.GetWorld"

type Request struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	GameStatus 	string
	InitialWorld [][]uint8
	Workers []string
}

type Response struct {
	World [][]uint8
}

type TurnRequest struct {
}

type TurnResponse struct {
	Turn int
	NumOfAliveCells int
}

type SdlResponse struct {
	Turn int
	AliveCells []util.Cell
}

type PauseRequest struct {
	Command string
}

type EmptyRequest struct {
}

type EmptyResponse struct {
}

type ResetRequest struct {
	LengthOfWorld int
}

type ResetResponse struct {
}

type NodeRequest struct {
	StartY int
	EndY int
	Width int
	CurrentWorld [][]uint8
}


type NodeResponse struct {
	WorldSlice [][]uint8
}

type WorldResponse struct {
	World [][]uint8
}




