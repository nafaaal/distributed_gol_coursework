package stubs

var TurnHandler = "GameOfLifeOperation.CompleteTurn"
var AliveCellGetter = "GameOfLifeOperation.GetAliveCell"

type Response struct {
	World [][]byte
}

type Request struct {
	Turns        int
	Threads      int
	ImageWidth   int
	ImageHeight  int
	InitialWorld [][]byte
}
