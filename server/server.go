package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
)

func getNumberOfNeighbours(p stubs.Request, col, row int, worldCopy func(y, x int) uint8) uint8 {
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

func calculateNextState(req stubs.Request, startY, endY int, worldCopy func(y, x int) uint8) [][]byte {
	height := endY - startY
	width := req.ImageWidth
	newWorld := makeMatrix(height, width)

	for col := 0; col < height; col++ {
		for row := 0; row < width; row++ {

			//startY+col gets the absolute y position when there is more than 1 worker
			n := getNumberOfNeighbours(req, startY+col, row, worldCopy)
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

func worker(req stubs.Request, startY, endY int, worldCopy func(y, x int) uint8, out chan<- [][]uint8) {
	newPixelData := calculateNextState(req, startY, endY, worldCopy)
	out <- newPixelData
}

func playTurn(req stubs.Request) [][]byte {
	worldCopy := makeImmutableMatrix(req.InitialWorld)
	var newPixelData [][]uint8
	if req.Threads == 1 {
		newPixelData = calculateNextState(req, 0, req.ImageHeight, worldCopy)
	} else {
		workerChannels := make([]chan [][]uint8, req.Threads)
		for i := 0; i < req.Threads; i++ {
			workerChannels[i] = make(chan [][]uint8)
		}

		workerHeight := req.ImageHeight / req.Threads

		for j := 0; j < req.Threads; j++ {
			startHeight := workerHeight * j
			endHeight := workerHeight * (j + 1)
			if j == req.Threads-1 { // send the extra part when workerHeight is not a whole number in last iteration
				endHeight += req.ImageHeight % req.Threads
			}
			go worker(req, startHeight, endHeight, worldCopy, workerChannels[j])
		}

		for k := 0; k < req.Threads; k++ {
			result := <-workerChannels[k]
			newPixelData = append(newPixelData, result...)
		}
	}

	return newPixelData
}

type GameOfLifeOperation struct{}

func (s *GameOfLifeOperation) CompleteTurn(req stubs.Request, res *stubs.Response) (err error) {
	res.World = playTurn(req)
	fmt.Println("Called Complete Turn - Server")
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

