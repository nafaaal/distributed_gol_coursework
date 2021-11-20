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

func writePgmData(p Params, c distributorChannels, world [][]uint8, turn int) {
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(turn)
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
	c.events <- ImageOutputComplete{turn, filename}
}

func findAliveCells(p Params, world [][]uint8) []util.Cell {
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

func timer(p Params, client *rpc.Client, c distributorChannels, finish *bool) {
	ticker := time.NewTicker(2 * time.Second)
	turnRequest := stubs.TurnRequest{}
	turnResponse := new(stubs.TurnResponse)
	for {
		<- ticker.C
		getAliveCells(client, turnRequest, turnResponse)
		if *finish {
			return
		}
		c.events <- AliveCellsCount{turnResponse.Turn, len(findAliveCells(p, turnResponse.CurrentWorld))}
	}
}

func saveWorld(p Params, c distributorChannels, client *rpc.Client){
	turnRequest := stubs.TurnRequest{}
	turnResponse := new(stubs.TurnResponse)
	getAliveCells(client, turnRequest, turnResponse)
	writePgmData(p, c, turnResponse.CurrentWorld, turnResponse.Turn)
}

func keyPressesFunc(p Params, c distributorChannels, client *rpc.Client, keyPresses <-chan rune, finished *bool, request stubs.Request){
	for {
		select {
		case key := <- keyPresses:
			if key == 's' {
				saveWorld(p, c, client)
			}
			if key == 'q' {
				fmt.Println("Closing Client...")
				response := stubs.EmptyResponse{}
				err := client.Call(stubs.Reset, request, &response)
				if err != nil {
					fmt.Println(err.Error())
				}
			}
			if key == 'k' {
				saveWorld(p, c, client)
				err := client.Call(stubs.Shutdown, stubs.EmptyRequest{}, &stubs.EmptyResponse{})
				//UNEXPECTED EOF
				if err != nil {
					fmt.Println(err.Error())
				}
			}
			if key == 'p' {
				fmt.Println("Pressed P")
			}
		}
	}
}



// distributor divides the work between workers and interacts with other goroutines.
// Also server keeps on going even after control C need to fix that
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

	server := "127.0.0.1:8030"
	client, _ := rpc.Dial("tcp", server)
	defer client.Close()

	initialWorld := makeMatrix(p.ImageHeight, p.ImageWidth)
	world := readPgmData(p, c, initialWorld)

	boolean := false

	request := stubs.Request{Turns: p.Turns, Threads: p.Threads, ImageWidth: p.ImageHeight, ImageHeight: p.ImageWidth, GameStatus: "NEW", InitialWorld: world}
	response := stubs.Response{World: makeMatrix(p.ImageWidth,p.ImageHeight)}

	go timer(p, client, c, &boolean)
	go keyPressesFunc(p, c, client, keyPresses, &boolean, request)

	// return end status somehow, and stop this call when q is pressed.
	makeCall(client, request, &response)
	boolean = true

	c.events <- FinalTurnComplete{p.Turns, findAliveCells(p, response.World)}
	writePgmData(p, c, response.World, p.Turns) // This line needed if out/ does not have files

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

//UNEXPECTED EOF
func makeCall(client *rpc.Client, req stubs.Request, res *stubs.Response) {
	err := client.Call(stubs.TurnHandler, req, res)
	if err != nil {
		fmt.Println(err)
	}

}

func getAliveCells(client *rpc.Client, req stubs.TurnRequest, res *stubs.TurnResponse) {
	err := client.Call(stubs.AliveCellGetter, req, res)
	if err != nil {
		fmt.Println(err)
	}
}