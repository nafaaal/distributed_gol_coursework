package stubs

var TurnHandler = "GameOfLifeOperation.CompleteTurn"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Response struct {
	World [][]byte
}

type Request struct {
	P Params
	InitialWorld [][]byte
}




