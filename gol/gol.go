package gol

import (
	"net/rpc"
)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	fname := make(chan string)
	out := make(chan uint8)
	in := make(chan uint8)
	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: fname,
		output:   out,
		input:    in,
	}
	go startIo(p, ioChannels)

	distributorChannels := distributorChannels{
		events:     events,
		ioCommand:  ioCommand,
		ioIdle:     ioIdle,
		ioFilename: fname,
		ioOutput:   out,
		ioInput:    in,

	}

	server := "127.0.0.1:8030"
	client, _ := rpc.Dial("tcp", server)
	defer client.Close()

	distributor(p, distributorChannels, keyPresses, client)
}
