package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

var turn int
var processGame bool
var world [][]uint8
var mutex sync.Mutex
var paused = make(chan int)
var resume = make(chan int)

var turnChannel = make(chan int)
var worldChannel = make(chan [][]uint8)

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func getNumberOfNeighbours(p stubs.Request, col, row int, worldCopy [][]uint8) uint8 {
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

func calculateNextState(p stubs.Request, worldCopy [][]uint8) [][]byte {
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

func resetState(worldSize int){
	mutex.Lock()
	turn = 0
	processGame = true
	world = makeMatrix(worldSize, worldSize)
	mutex.Unlock()
}


type GameOfLifeOperation struct{}


func (s *GameOfLifeOperation) CompleteTurn(req stubs.Request, res *stubs.Response) (err error) {
	if req.GameStatus == "NEW" ||  req.GameStatus == "TEST" {
		resetState(req.ImageWidth)
	}
	world = req.InitialWorld
	for turn < req.Turns && processGame {
		mutex.Lock()
		world = calculateNextState(req, world)
		turn++

		if req.GameStatus == "NEW" {
			worldChannel <- world
			turnChannel <- turn
		}

		mutex.Unlock()

		select {
		case  <- paused:
			<-resume
		default:
			break
		}

	}
	res.World = world
	return
}

func (s *GameOfLifeOperation) GetAliveCell(req stubs.EmptyRequest, res *stubs.TurnResponse) (err error) {
	mutex.Lock()
	res.Turn = turn
	res.CurrentWorld = world
	mutex.Unlock()
	return
}

func (s *GameOfLifeOperation) Shutdown(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error) {
	fmt.Println("Exiting...")
	processGame = false
	<- time.After(1*time.Second)
	os.Exit(0)
	return
}

func (s *GameOfLifeOperation) PauseAndResume(req stubs.PauseRequest, res *stubs.EmptyResponse) (err error) {
	if req.Command == "PAUSE" {
		paused <- 1
	}
	if req.Command == "RESUME"{
		resume <- 1
	}
	return
}


func (s *GameOfLifeOperation) ResetState(req stubs.ResetRequest, res *stubs.EmptyResponse) (err error) {
	processGame = false
	return
}

func (s *GameOfLifeOperation) GetWorldPerTurn(req stubs.EmptyRequest, res *stubs.TurnResponse) (err error) {
	for i := 0; i < 2; i++ {
		select {
		case turn := <- turnChannel:
			res.Turn = turn
		case world := <- worldChannel:
			res.CurrentWorld = world
		}
	}
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&GameOfLifeOperation{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)

	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			fmt.Println("Error in listerner")
		}
	}(listener)

	rpc.Accept(listener)

}
