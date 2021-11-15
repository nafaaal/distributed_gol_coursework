package stubs

var TurnHandler = "GameOfLifeOperation.CompleteTurn"


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
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	InitialWorld [][]byte
}




