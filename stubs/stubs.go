package stubs

var TurnHandler = "GameOfLifeOperation.CompleteTurn"
var AliveCellGetter = "GameOfLifeOperation.GetAliveCell"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Response struct {
	World [][]byte
}

type TurnResponse struct {
	Turn int
	CellCount int
}

type TurnRequest struct {
}

type Request struct {
	P Params
	InitialWorld [][]byte
}




