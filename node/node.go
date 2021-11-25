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
var first_halo_channel = make(chan []uint8)
var last_halo_channel = make(chan []uint8)
var in_halo = make(chan *stubs.HaloResponse)


type Node struct{}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

//func findAliveCellCount(world [][]uint8, req stubs.NodeRequest) int {
//	height := req.EndY - req.StartY
//	width := req.Width
//	var count = 0
//	for col := 0; col < height; col++ {
//		for row := 0; row < width; row++ {
//			if world[req.StartY+col][row] == 255 {
//				count++
//			}
//		}
//	}
//	return count
//}

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
				height := (col + p.EndY-p.StartY + i) % (p.EndY-p.StartY)
				width := (row + p.Width + j) % p.Width
				//fmt.Printf("Height inside neighbours = %d,  endy-starty = %d\n", height , p.EndY-p.StartY )
				if worldCopy[height][width] == 255 {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func calculateNextState(req stubs.NodeRequest, initialWorld [][]uint8) [][]uint8 {
	//height := req.EndY - req.StartY
	//width := req.Width
	height := len(req.CurrentWorld)
	width := len(req.CurrentWorld[0])
	newWorld := makeMatrix(height, width)

	fmt.Printf("HEIGHT IS - %d\n", height)
	fmt.Printf("WIDTH IS - %d\n", width)
	for col := 1; col < height-1; col++ {
		for row := 0; row < width; row++ {

			//fmt.Printf("len of initial world inside calculate next step- %d", len(initialWorld))
			n := getNumberOfNeighbours(req, col, row, initialWorld)
			currentState := initialWorld[col][row]

			if currentState == 255 {
				if n == 2 || n == 3 {
					newWorld[col-1][row] = 255
				}
			}

			if currentState == 0 {
				if n == 3 {
					newWorld[col-1][row] = 255
				}
			}
		}
	}
	fmt.Println("newworld returned")
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

//ProcessSlice treat slice as the whole world?
func (s *Node) ProcessSlice(req stubs.NodeRequest, res *stubs.NodeResponse) (err error) {
	world = req.CurrentWorld
	for turn := 1; turn < req.Turns+1; turn++{
		var nextWorld [][]uint8
		var neighboursWorld [][]uint8

		select {
		case halo := <- in_halo:
			neighboursWorld = append(neighboursWorld, halo.FirstHalo)
			neighboursWorld = append(neighboursWorld, world...)
			neighboursWorld = append(neighboursWorld, halo.LastHalo)
		default:
			neighboursWorld = append(neighboursWorld, world...)
		}
		nextWorld = calculateNextState(req, neighboursWorld)

		mutex.Lock()
		fmt.Println("1")
		flippedCellChannels <- flippedCells(world, nextWorld)
		fmt.Println("2")
		aliveCellCountChannel <- findAliveCellCount(nextWorld)
		fmt.Println("3")
		first_halo_channel <- nextWorld[0]
		fmt.Println("4")
		last_halo_channel <- nextWorld[len(world)-1]
		fmt.Println("5")

		turnChannel <- turn
		world = nextWorld

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


func (s *Node) GetTurnAndAliveCell(req stubs.EmptyRequest, res *stubs.TurnResponse) (err error) {
	for i := 0; i<2; i++ {
		select {
	case turn := <- turnChannel:
		res.Turn = turn
	case count := <- aliveCellCountChannel:
		res.NumOfAliveCells = count
	}
	}
	return
}

func (s *Node) GetHaloRegions(req stubs.EmptyRequest, res *stubs.HaloResponse) (err error) {
	for i := 0; i<2; i++ {
		select {
		case first := <- first_halo_channel:
			res.FirstHalo = first
		case last := <- last_halo_channel:
			res.LastHalo = last
		}
	}
	return
}


func (s *Node) ReceiveHaloRegions(req stubs.EmptyRequest, res *stubs.HaloResponse) (err error) {
	in_halo <- res
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