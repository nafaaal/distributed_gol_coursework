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


type Node struct{}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func findAliveCellCount(world [][]uint8) int {
	var length = len(world)
	var count = 0
	for col := 0; col < length; col++ {
		for row := 0; row < length; row++ {
			if world[col][row] == 255 {
				count++
			}
		}
	}
	return count
}

func getNumberOfNeighbours(p stubs.NodeRequest, col, row int, worldCopy [][]uint8) uint8 {
	var neighbours uint8
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			if i != 0 || j != 0 { //{i=0, j=0} is the cell you are trying to get neighbours of!
				height := (col + p.Width + i) % p.Width // NEED TO CHANGE to height
				width := (row + p.Width + j) % p.Width
				if worldCopy[height][width] == 255 {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func calculateNextState(req stubs.NodeRequest, initialWorld [][]uint8) [][]uint8 {
	height := req.EndY - req.StartY
	width := req.Width
	newWorld := makeMatrix(height, width)

	for col := 0; col < height; col++ {
		for row := 0; row < width; row++ {

			//startY+col gets the absolute y position when there is more than 1 worker
			n := getNumberOfNeighbours(req, req.StartY+col, row, initialWorld)
			currentState := initialWorld[req.StartY+col][row]

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
func flippedCells(initial, nextState [][]uint8) []util.Cell{
	length := len(initial)
	var flipped []util.Cell
	for col := 0; col < length; col++ {
		for row := 0; row < length; row++ {
			if initial[col][row] != nextState[col][row]{
				flipped = append(flipped, util.Cell{X: row, Y: col})
			}
		}
	}
	return flipped
}

func (s *Node) ProcessSlice(req stubs.NodeRequest, res *stubs.NodeResponse) (err error) {
	world = req.CurrentWorld
	//fmt.Printf("0,%d \n", findAliveCellCount(world))
	//fmt.Println(len(world))
	for turn := 0; turn < req.Turns; turn++{
		//fmt.Printf("%d,%d \n",turn, findAliveCellCount(world))
		var nextWorld [][]uint8
		nextWorld = calculateNextState(req, world)
		mutex.Lock()

		fmt.Println("1")
		flippedCellChannels <- flippedCells(world, nextWorld)
		fmt.Println("2")
		temp := <- aliveCellCountChannel
		fmt.Println(temp)
		aliveCellCountChannel <- findAliveCellCount(nextWorld)
		fmt.Println("3")
		turnChannel <- turn
		fmt.Println("4")
		world = nextWorld
		fmt.Println("5")

		mutex.Unlock()
	}
	res.WorldSlice = world
	return
}

func (s *Node) GetFlippedCells(req stubs.EmptyRequest, res *stubs.FlippedCellResponse) (err error) {
	select {
	case flipped := <- flippedCellChannels:
		res.FlippedCells = flipped
	}
	return
}

func (s *Node) GetAliveCellCount(req stubs.EmptyRequest, res *stubs.AliveCellCountResponse) (err error) {
	select {
	case count := <- aliveCellCountChannel:
		res.Count = count
	}
	return
}

func (s *Node) GetTurn(req stubs.EmptyRequest, res *stubs.TurnResponse) (err error) {
	select {
	case turn := <- turnChannel:
		res.Turn = turn
	}
	return
}


func main() {
	pAddr := flag.String("port", "8031", "Port to listen on")
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