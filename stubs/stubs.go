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

type TurnRequest struct {
}

type TurnResponse struct {
	Turn int
	CellCount int
}

type Request struct {
	P Params
	InitialWorld [][]byte
}




