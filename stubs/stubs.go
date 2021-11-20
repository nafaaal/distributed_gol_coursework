package stubs

var TurnHandler = "GameOfLifeOperation.CompleteTurn"
var AliveCellGetter = "GameOfLifeOperation.GetAliveCell"
var Shutdown = "GameOfLifeOperation.Shutdown"
var Reset = "GameOfLifeOperation.ResetState"

type Request struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	GameStatus 	string
	InitialWorld [][]uint8
}

type Response struct {
	World [][]uint8
}

type TurnRequest struct {
}

type TurnResponse struct {
	Turn int
	CurrentWorld [][]uint8
}

type EmptyRequest struct {
}

type EmptyResponse struct {
}

type ResetRequest struct {
}

type ResetResponse struct {
}





