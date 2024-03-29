package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var world [][]uint8
var mutex sync.Mutex
var flippedCellChannels = make(chan []util.Cell)
var aliveCellCountChannel = make(chan int)
var turnChannel = make(chan int)
var outHalo = make(chan stubs.HaloResponse)
var inHalo = make(chan stubs.HaloResponse)
var paused = make(chan int)
var resume = make(chan int)

type Node struct{}

func calculateNeighbours(width, x, y int, haloWorld [][]uint8) int {
	height := len(haloWorld)
	neighbours := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i != 0 || j != 0 {
				h := (y + height + i) % height
				w := (x + width + j) % width
				if haloWorld[h][w] == 255 {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func calculateNextState(height, width int, haloWorld [][]uint8) [][]uint8 {

	newWorld := makeMatrix(height, width)

	for c0, c2 := 1, 0; c0 < height+1; c0, c2 = c0+1, c2+1 {
		for r := 0; r < width; r++ {

			neighbours := calculateNeighbours(width, r, c0, haloWorld)
			currentState := haloWorld[c0][r]

			if currentState == 255 {
				if neighbours == 2 || neighbours == 3 {
					newWorld[c2][r] = 255
				}
			}
			if currentState == 0 {
				if neighbours == 3 {
					newWorld[c2][r] = 255
				}
			}
		}
	}
	return newWorld
}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func findAliveCellCount(height, width int, world [][]uint8) int {
	var count = 0
	for col := 0; col < height; col++ {
		for row := 0; row < width; row++ {
			if world[col][row] == 255 {
				count++
			}
		}
	}
	return count
}

func flippedCells(height, width, startY int, initial, nextState [][]uint8) []util.Cell {
	var flipped []util.Cell
	for col := 0; col < height; col++ {
		for row := 0; row < width; row++ {
			if initial[col][row] != nextState[col][row] {
				flipped = append(flipped, util.Cell{X: row, Y: startY + col})
			}
		}
	}
	return flipped
}

func (s *Node) ProcessSlice(req stubs.NodeRequest, res *stubs.NodeResponse) (err error) {
	world = req.CurrentWorld
	for turn := 1; turn < req.Turns+1; turn++ {

		var nextWorld [][]uint8
		var neighboursWorld [][]uint8

		select {
		case halo := <-inHalo:
			neighboursWorld = append(neighboursWorld, halo.FirstHalo)
			neighboursWorld = append(neighboursWorld, world...)
			neighboursWorld = append(neighboursWorld, halo.LastHalo)
		}

		worldHeight := len(world)
		worldWidth := req.Width
		nextWorld = calculateNextState(worldHeight, worldWidth, neighboursWorld)

		mutex.Lock()

		flippedCellChannels <- flippedCells(worldHeight, worldWidth, req.StartY, world, nextWorld)
		world = nextWorld
		aliveCellCountChannel <- findAliveCellCount(worldHeight, worldWidth, world)
		outHalo <- stubs.HaloResponse{FirstHalo: world[0], LastHalo: world[len(nextWorld)-1]}
		turnChannel <- turn

		mutex.Unlock()

		select {
		case  <- paused:
			<-resume
		default:
			break
		}

	}
	res.WorldSlice = world
	return
}

func (s *Node) GetFlippedCells(req stubs.EmptyRequest, res *stubs.FlippedCellResponse) (err error) {
	select {
	case flipped := <-flippedCellChannels:
		res.FlippedCells = flipped
	}
	return
}

func (s *Node) GetTurnAndAliveCell(req stubs.EmptyRequest, res *stubs.TurnResponse) (err error) {
	for i := 0; i < 2; i++ {
		select {
		case turn := <-turnChannel:
			res.Turn = turn
		case count := <-aliveCellCountChannel:
			res.NumOfAliveCells = count
		}
	}
	return
}

func (s *Node) SendHaloToBroker(req stubs.EmptyRequest, res *stubs.HaloResponse) (err error) {
	select {
	case halo := <-outHalo:
		res.FirstHalo = halo.FirstHalo
		res.LastHalo = halo.LastHalo
	}
	return
}

func (s *Node) SendHaloToNode(haloFromBroker stubs.HaloResponse, res *stubs.EmptyResponse) (err error) {
	inHalo <- haloFromBroker
	return
}

func (s *Node) PauseAndResumeNode(req stubs.PauseRequest, res *stubs.EmptyResponse) (err error) {
	if req.Command == "PAUSE" {
		paused <- 1
	}
	if req.Command == "RESUME"{
		resume <- 1
	}
	return
}

func (s *Node) GetNode(req stubs.EmptyRequest, res *stubs.NodeResponse) (err error) {
	mutex.Lock()
	res.WorldSlice = world
	mutex.Unlock()
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Node{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)

	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			fmt.Println("Error in listerner")
		}
	}(listener)
	rpc.Accept(listener)

}
