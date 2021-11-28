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

func timer(client *rpc.Client, c distributorChannels, finish *bool) {
	ticker := time.NewTicker(2 * time.Second)
	for {
		<-ticker.C
		if !(*finish) {
			turn, aliveCellCount := callTurnAndWorld(client)
			//fmt.Println(turn, aliveCellCount)
			c.events <- AliveCellsCount{turn, aliveCellCount}
		} else {
			break
		}
	}
	return
}

func saveWorld(p Params, c distributorChannels, client *rpc.Client) {
	turn, _ := callTurnAndWorld(client)
	world := callWorld(client)
	writePgmData(p, c, world, turn)
}

func stateChange(client *rpc.Client, c distributorChannels, newState State) {
	turn, _ := callTurnAndWorld(client)
	c.events <- StateChange{turn, newState}
}

func keyPressesFunc(p Params, c distributorChannels, client *rpc.Client, keyPresses <-chan rune) {
	for {
		select {
		case key := <-keyPresses:
			if key == 's' {
				saveWorld(p, c, client)
			}
			if key == 'q' {
				fmt.Println("Closing Client...")
				err := client.Call(stubs.Reset, stubs.EmptyRequest{}, &stubs.EmptyResponse{})
				if err != nil {
					fmt.Println(err.Error())
				}
			}
			if key == 'k' {
				saveWorld(p, c, client)
				stateChange(client, c, Quitting)
				err := client.Call(stubs.Shutdown, stubs.EmptyRequest{}, &stubs.EmptyResponse{})
				if err != nil {
					fmt.Println(err.Error())
				}

			}
			if key == 'p' {
				fmt.Println("Pressed P")
				callPauseAndResume(client, stubs.PauseRequest{Command: "PAUSE"})
				stateChange(client, c, Paused)
				for {
					await := <-keyPresses
					if await == 'p' {
						callPauseAndResume(client, stubs.PauseRequest{Command: "RESUME"})
						stateChange(client, c, Executing)
						break
					}
				}
			}
		}
	}
}

func sdlHandler(p Params, c distributorChannels, client *rpc.Client) {

	for i := 0; i < p.Turns; i++ {
		response := new(stubs.SdlResponse)
		err := client.Call(stubs.GetWorldPerTurn, stubs.EmptyRequest{}, response)
		if err != nil {
			fmt.Println(err)
		}

		for _, flippedCells := range response.FlippedCells {
			c.events <- CellFlipped{CompletedTurns: response.Turn, Cell: flippedCells}
		}
		c.events <- TurnComplete{response.Turn}
	}
	return
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

	brokerAddress := "localhost:8003"
	client, _ := rpc.Dial("tcp", brokerAddress)
	defer client.Close()

	initialWorld := readPgmData(p, c, makeMatrix(p.ImageHeight, p.ImageWidth))

	allTurnsProcessed := false
	go timer(client, c, &allTurnsProcessed)
	go keyPressesFunc(p, c, client, keyPresses)
	go sdlHandler(p, c, client)

	//var nodeAddresses []string
	//for _, node := range strings.Split(Server, ",") {
	//	nodeAddresses = append(nodeAddresses, node+":8030")
	//}
	//var testNodes = []string{"localhost:8000","localhost:8001","localhost:8004"}
	//var testNodes = []string{"localhost:8000", "localhost:8001"}
	var testNodes = []string{"localhost:8030","localhost:8031"}
	//var testNodes = []string{"localhost:8030"}

	request := stubs.Request{Turns: p.Turns, Threads: p.Threads, ImageWidth: p.ImageHeight, ImageHeight: p.ImageWidth, GameStatus: "NEW", InitialWorld: initialWorld, Workers: testNodes}
	response := stubs.Response{World: makeMatrix(p.ImageWidth, p.ImageHeight)}

	callTurn(client, request, &response)
	allTurnsProcessed = true

	//respone.world needs to be good
	c.events <- FinalTurnComplete{p.Turns, findAliveCells(p, response.World)}
	writePgmData(p, c, response.World, p.Turns) // This line needed if out/ does not have files

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func callTurn(client *rpc.Client, req stubs.Request, res *stubs.Response) {
	err := client.Call(stubs.TurnHandler, req, res)
	if err != nil {
		fmt.Println(err)
	}
}

func callTurnAndWorld(client *rpc.Client) (int, int) {
	turnRequest := stubs.TurnRequest{}
	turnResponse := new(stubs.TurnResponse)
	err := client.Call(stubs.AliveCellGetter, turnRequest, turnResponse)
	if err != nil {
		fmt.Println(err)
	}
	return turnResponse.Turn, turnResponse.NumOfAliveCells
}

func callPauseAndResume(client *rpc.Client, req stubs.PauseRequest) {
	err := client.Call(stubs.PauseAndResume, req, &stubs.EmptyResponse{})
	if err != nil {
		fmt.Println(err)
	}
}

func callWorld(client *rpc.Client) [][]uint8 {
	worldResponse := new(stubs.WorldResponse)
	err := client.Call(stubs.GetWorld, stubs.EmptyRequest{}, worldResponse)
	if err != nil {
		fmt.Println(err)
	}
	return worldResponse.World
}
