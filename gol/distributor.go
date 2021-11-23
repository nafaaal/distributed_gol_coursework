package gol

import (
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}


// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

	brokerAddress := "localhost:8082"

	client, _ := rpc.Dial("tcp", brokerAddress)
	defer client.Close()

	initialWorld := readPgmData(p, c, makeMatrix(p.ImageHeight, p.ImageWidth))

	allTurnsProcessed := false
	go timer(p, client, c, &allTurnsProcessed)
	go keyPressesFunc(p, c, client, keyPresses)
	go sdlHandler(p, c, client, initialWorld)

	request := stubs.Request{Turns: p.Turns, Threads: p.Threads, ImageWidth: p.ImageHeight, ImageHeight: p.ImageWidth, GameStatus: "NEW", InitialWorld: initialWorld}
	response := stubs.Response{World: makeMatrix(p.ImageWidth,p.ImageHeight)}

	callTurn(client, request, &response)
	allTurnsProcessed = true

	c.events <- FinalTurnComplete{p.Turns, findAliveCells(p, response.World)}
	writePgmData(p, c, response.World, p.Turns) // This line needed if out/ does not have files

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
