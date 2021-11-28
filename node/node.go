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
var firstHaloChannel = make(chan []uint8)
var lastHaloChannel = make(chan []uint8)
var inHalo = make(chan stubs.HaloResponse)

type Node struct{}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func findAliveCellCount(world [][]uint8) int {
	var height = len(world)
	var width = len(world[0])
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

func getNumberOfNeighbours(p stubs.NodeRequest, col, row int, worldCopy func(y, x int) uint8) uint8 {
	var neighbours uint8
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			if i != 0 || j != 0 { //{i=0, j=0} is the cell you are trying to get neighbours of!
				height := (col + p.EndY - p.StartY + i) % (p.EndY - p.StartY)
				width := (row + p.Width + j) % p.Width
				// fmt.Printf("Height = %d, Width = %d, endy-starty = %d\n", height, width, p.EndY-p.StartY)
				if worldCopy(height, width) == 255 {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func makeImmutableMatrix(matrix [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return matrix[y][x]
	}
}


func calculateNextState(req stubs.NodeRequest, initialWorld [][]uint8) [][]uint8 {
	height := len(req.CurrentWorld)
	width := len(req.CurrentWorld[0])
	newWorld := makeMatrix(height, width) //original slice size

	//fmt.Println(height)
	//fmt.Println(len(initialWorld))

	neighbours := makeImmutableMatrix(initialWorld) // the one with halo

	for col, col2 := 1, 0; col2 < height; col, col2 = col+1, col2+1 {
	//for col := 1 ; col < height-1; col= col+1 {
			for row := 0; row < width; row++ {

				//fmt.Printf("len of initial world inside calculate next step- %d", len(initialWorld))
				n := getNumberOfNeighbours(req, col, row, neighbours)
				currentState := initialWorld[col][row]

				if currentState == 255 {
					if n == 2 || n == 3 {
						newWorld[col2][row] = 255
					}
				}

				if currentState == 0 {
					if n == 3 {
						newWorld[col2][row] = 255
					}
				}
			}
		}

	fmt.Println(len(newWorld))
	return newWorld
}
func flippedCells(req stubs.NodeRequest, initial, nextState [][]uint8) []util.Cell {
	length := len(initial)
	height := len(initial[0])
	var flipped []util.Cell
	for col := 0; col < length; col++ {
		for row := 0; row < height; row++ {
			if initial[col][row] != nextState[col][row] {
				flipped = append(flipped, util.Cell{X: row, Y: req.StartY+col})
			}
		}
	}
	return flipped
}

//ProcessSlice treat slice as the whole world?
func (s *Node) ProcessSlice(req stubs.NodeRequest, res *stubs.NodeResponse) (err error) {
	world = req.CurrentWorld
	for turn := 1; turn < req.Turns+1; turn++ {
		var nextWorld [][]uint8
		var neighboursWorld [][]uint8
		var h1, h2 []uint8

		select {
		case halo := <-inHalo: //issue is send empty halos
			neighboursWorld = append(neighboursWorld, halo.FirstHalo)
			neighboursWorld = append(neighboursWorld, world...)
			neighboursWorld = append(neighboursWorld, halo.LastHalo)
			//default:
			//	neighboursWorld = append(neighboursWorld, world...)
		}

		nextWorld = calculateNextState(req, neighboursWorld)
		//fmt.Printf("turn-%d, alivecellcount-%d, ", turn, findAliveCellCount(nextWorld))

		mutex.Lock()
		flippedCellChannels <- flippedCells(req, world, nextWorld)
		aliveCellCountChannel <- findAliveCellCount(nextWorld)
		h1 = nextWorld[0]
		firstHaloChannel <- h1
		h2 = nextWorld[len(nextWorld)-1]
		lastHaloChannel <- h2

		fmt.Printf("%d %d ", countSlice(h1), countSlice(h2))

		turnChannel <- turn
		world = nextWorld

		mutex.Unlock()
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

func (s *Node) GetAliveCellCount(req stubs.EmptyRequest, res *stubs.AliveCellCountResponse) (err error) {
	select {
	case count := <-aliveCellCountChannel:
		res.Count = count
	}
	return
}

func (s *Node) GetTurn(req stubs.EmptyRequest, res *stubs.TurnResponse) (err error) {
	select {
	case turn := <-turnChannel:
		res.Turn = turn
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

func (s *Node) GetHaloRegions(req stubs.EmptyRequest, res *stubs.HaloResponse) (err error) {
	for i := 0; i < 2; i++ {
		select {
		case first := <-firstHaloChannel:
			res.FirstHalo = first
			//fmt.Println(first)
		case last := <-lastHaloChannel:
			res.LastHalo = last //this giving nil
			//fmt.Println(last)
		}
	}
	return
}

func countSlice(arr []uint8) int{
	count := 0
	for _, element := range arr {
		if element>0{
			count++
		}
	}
	return count
}


func countHalos(arr stubs.HaloResponse){
	top := 0
	down := 0
	top += countSlice(arr.FirstHalo)
	down += countSlice(arr.LastHalo)
	fmt.Printf("%d %d\n", top, down)
}

func (s *Node) SendHaloToNode(req stubs.HaloResponse, res *stubs.EmptyResponse) (err error) {
	//countHalos(req)
	inHalo <- req
	return
}

func main() {
	pAddr := flag.String("port", "8000", "Port to listen on")
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
