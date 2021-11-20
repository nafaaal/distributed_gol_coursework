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
	InitialWorld [][]byte
}

type Response struct {
	World [][]byte
}

type TurnRequest struct {
}

type TurnResponse struct {
	Turn int
	CellCount int
}

type EmptyRequest struct {
}

type EmptyResponse struct {
}

type ResetRequest struct {
}

type ResetResponse struct {
}





