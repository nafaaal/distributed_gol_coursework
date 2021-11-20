package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var turn int
var world [][]uint8
var mutex sync.Mutex
var processGame bool = true

func findAliveCells(world [][]byte) []util.Cell {
	var alive []util.Cell
	for col := 0; col <len(world); col++ {
		for row := 0; row <len(world); row++ {
			if world[col][row] == 255 {
				alive = append(alive, util.Cell{X: row, Y: col})
			}
		}
	}
	return alive
}

func getNumberOfNeighbours(p stubs.Params, col, row int, worldCopy [][]uint8) uint8 {
	var neighbours uint8
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			if i != 0 || j != 0 { //{i=0, j=0} is the cell you are trying to get neighbours of!
				height := (col + p.ImageHeight + i) % p.ImageHeight
				width := (row + p.ImageWidth + j) % p.ImageWidth
				if worldCopy[height][width] == 255 {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func calculateNextState(p stubs.Params, worldCopy [][]uint8) [][]byte {
	height := p.ImageHeight
	width := p.ImageWidth
	newWorld := makeMatrix(height, width)

	for col := 0; col < height; col++ {
		for row := 0; row < width; row++ {

			//startY+col gets the absolute y position when there is more than 1 worker
			n := getNumberOfNeighbours(p, col, row, worldCopy)
			currentState := worldCopy[col][row]

			if currentState == 255 {
				if n == 2 || n == 3 {
					newWorld[col][row] = 255
				}
			}

			if currentState == 0 {
				if n == 3 {
					newWorld[col][row] = 255
				}
			}
		}
	}
	return newWorld
}


// distributor divides the work between workers and interacts with other goroutines.
//needs to stop when like something happens idk
func distributor(req stubs.Request, res *stubs.Response) [][]uint8 {


	world = req.InitialWorld
	for turn < req.P.Turns && processGame {
		mutex.Lock()
		world = calculateNextState(req.P, world)
		turn++
		mutex.Unlock()
	}
	return world
}


type GameOfLifeOperation struct{}

func printStats(end *chan bool){
	ticker := time.NewTicker(5*time.Second)
	for {
		select {
		case <- *end:
			return
		default:
			<- ticker.C
			fmt.Println(turn)
			fmt.Println(len(findAliveCells(world)))
		}

	}

}

func resetState(req stubs.Request){
	mutex.Lock()
	turn = 0
	world = makeMatrix(req.P.ImageWidth, req.P.ImageWidth)
	mutex.Unlock()
}


func (s *GameOfLifeOperation) CompleteTurn(req stubs.Request, res *stubs.Response) (err error) {
	end := new(chan bool)
	go printStats(end)
	if req.P.GameStatus == "NEW"{
		resetState(req)
	}
	processGame = true
	res.World = distributor(req, res)
	return
}

func (s *GameOfLifeOperation) GetAliveCell(req stubs.TurnRequest, res *stubs.TurnResponse) (err error) {
	fmt.Println("Called Alive Cells - Server")
	mutex.Lock()
	res.Turn = turn
	res.CellCount = len(findAliveCells(world))
	mutex.Unlock()
	return
}

func (s *GameOfLifeOperation) Shutdown(req stubs.Request, res *stubs.Response) (err error) {
	os.Exit(0)
	return
}

func (s *GameOfLifeOperation) ResetState(req stubs.Request, res *stubs.Response) (err error) {
	fmt.Println("STATE RSET FUNCTION CALLED????")
	processGame = false
	resetState(req)
	return
}

func main() {
	//http.ListenAndServe("localhost:8030", nil) // this gives some error wtf
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&GameOfLifeOperation{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)


	defer listener.Close()
	rpc.Accept(listener)
}

