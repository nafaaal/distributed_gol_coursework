package stubs

var TurnHandler = "GameOfLifeOperation.CompleteTurn"
var AliveCellGetter = "GameOfLifeOperation.GetAliveCell"
var Shutdown = "GameOfLifeOperation.Shutdown"
var Reset = "GameOfLifeOperation.ResetState"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	GameStatus 	string
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

type ShutDownRequest struct {
}

type ShutDownResponse struct {
}

type ResetRequest struct {
}

type ResetResponse struct {
}

type Request struct {
	P Params
	InitialWorld [][]byte
}




