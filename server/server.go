package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/util"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var turn int
var world [][]byte
//var mutex sync.Mutex

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

func getNumberOfNeighbours(p stubs.Params, col, row int, worldCopy func(y, x int) uint8) uint8 {
	var neighbours uint8
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			if i != 0 || j != 0 { //{i=0, j=0} is the cell you are trying to get neighbours of!
				height := (col + p.ImageHeight + i) % p.ImageHeight
				width := (row + p.ImageWidth + j) % p.ImageWidth
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

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func calculateNextState(p stubs.Params, startY, endY int, worldCopy func(y, x int) uint8) [][]byte {
	height := endY - startY
	width := p.ImageWidth
	newWorld := makeMatrix(height, width)

	for col := 0; col < height; col++ {
		for row := 0; row < width; row++ {

			//startY+col gets the absolute y position when there is more than 1 worker
			n := getNumberOfNeighbours(p, startY+col, row, worldCopy)
			currentState := worldCopy(startY+col, row)

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

func worker(p stubs.Params, startY, endY int, worldCopy func(y, x int) uint8, out chan<- [][]uint8) {
	newPixelData := calculateNextState(p, startY, endY, worldCopy)
	out <- newPixelData
}

func playTurn(p stubs.Params, world [][]byte) [][]byte {
	worldCopy := makeImmutableMatrix(world)
	var newPixelData [][]uint8
	if p.Threads == 1 {
		newPixelData = calculateNextState(p, 0, p.ImageHeight, worldCopy)
	} else {
		workerChannels := make([]chan [][]uint8, p.Threads)
		for i := 0; i < p.Threads; i++ {
			workerChannels[i] = make(chan [][]uint8)
		}

		workerHeight := p.ImageHeight / p.Threads

		for j := 0; j < p.Threads; j++ {
			startHeight := workerHeight * j
			endHeight := workerHeight * (j + 1)
			if j == p.Threads-1 { // send the extra part when workerHeight is not a whole number in last iteration
				endHeight += p.ImageHeight % p.Threads
			}
			go worker(p, startHeight, endHeight, worldCopy, workerChannels[j])
		}

		for k := 0; k < p.Threads; k++ {
			result := <-workerChannels[k]
			newPixelData = append(newPixelData, result...)
		}
	}

	return newPixelData
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(req stubs.Request, res *stubs.Response) {

	turn = 0
	world = req.InitialWorld
	for turn < req.P.Turns {
		//mutex.Lock()
		world = playTurn(req.P, world)
		//mutex.Unlock()
		turn++
	}
	res.World = world

}

type GameOfLifeOperation struct{}

func (s *GameOfLifeOperation) CompleteTurn(req stubs.Request, res *stubs.Response) (err error) {
	//distributor(req, res)
	res.World = playTurn(req.P, req.InitialWorld)
	fmt.Println("Called Complete Turn - Server")
	return
}

func (s *GameOfLifeOperation) GetAliveCell(req stubs.TurnRequest, res *stubs.TurnResponse) (err error) {
	fmt.Println("Called Alive Cells - Server")
	//mutex.Lock()
	res.Turn = turn
	res.CellCount = len(findAliveCells(world))
	//mutex.Unlock()
	return
}

func main() {
	//http.ListenAndServe("localhost:8030", nil) // this gives some error wtf
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	//rand.Seed(time.Now().UnixNano())
	rpc.Register(&GameOfLifeOperation{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}

