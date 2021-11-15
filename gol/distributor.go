package gol

import (
	"fmt"
	"net/rpc"
	"strconv"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}


func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func readPgmData(p Params, c distributorChannels, world [][]uint8) [][]uint8 {
	c.ioCommand <- ioInput
	c.ioFilename <- strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	for col := 0; col < p.ImageHeight; col++ {
		for row := 0; row < p.ImageWidth; row++ {
			data := <-c.ioInput
			world[col][row] = data
			if data == 255 {
				c.events <- CellFlipped{0, util.Cell{X: row, Y: col}}
			}
		}
	}
	return world
}

func writePgmData(p Params, c distributorChannels, world [][]uint8) {
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Turns)
	c.ioCommand <- ioOutput
	c.ioFilename <- filename
	for col := 0; col < p.ImageHeight; col++ {
		for row := 0; row < p.ImageWidth; row++ {
			if world[col][row] == 255 {
				c.ioOutput <- 255
			} else {
				c.ioOutput <- 0
			}
		}
	}
	c.events <- ImageOutputComplete{p.Turns, filename}
}

func findAliveCells(p Params, world [][]byte) []util.Cell {
	var alive []util.Cell
	for col := 0; col < p.ImageHeight; col++ {
		for row := 0; row < p.ImageWidth; row++ {
			if world[col][row] == 255 {
				alive = append(alive, util.Cell{X: row, Y: col})
			}
		}
	}
	return alive
}

func timers(client *rpc.Client, ticker *time.Ticker, c distributorChannels, p Params) {
	for {
		<- ticker.C
		turnRequest := stubs.TurnRequest{}
		turnResponse := new(stubs.TurnResponse)
		getTurn(client, turnRequest, turnResponse)
		c.events <- AliveCellsCount{turnResponse.Turn, turnResponse.CellCount}
	}
}



// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune, ) {

	initialWorld := makeMatrix(p.ImageHeight, p.ImageWidth)
	world := readPgmData(p, c, initialWorld)
	var response *stubs.Response

	server := "127.0.0.1:8030"
	client, _ := rpc.Dial("tcp", server)
	defer client.Close()

	ticker := time.NewTicker(2 * time.Second)
	go timers(client, ticker, c, p)

	response = new(stubs.Response)
	makeCall(p, world, client, response)


	c.events <- FinalTurnComplete{p.Turns, findAliveCells(p, response.World)}
	writePgmData(p, c, response.World) // This line needed if out/ does not have files

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func makeCall(p Params, world [][]uint8, client *rpc.Client, response *stubs.Response) {
	request := stubs.Request{Turns: p.Turns, Threads: p.Threads, ImageWidth: p.ImageWidth, ImageHeight: p.ImageHeight, InitialWorld: world}
	err := client.Call(stubs.TurnHandler, request, response)
	if err != nil {
		fmt.Println("make call oof")
	}
}

func getTurn(client *rpc.Client, req stubs.TurnRequest, res *stubs.TurnResponse) {
	err := client.Call(stubs.TurnHandler, req, res)
	if err != nil {
		fmt.Println("get turn oof")
	}
}
