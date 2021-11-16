package stubs

var TurnHandler = "GameOfLifeOperation.CompleteTurn"

type Response struct {
	World [][]byte
}

type Request struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	InitialWorld [][]byte
}




