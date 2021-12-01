package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type Node struct{}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

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

func (s *Node) ProcessSlice(req stubs.NodeRequest, res *stubs.NodeResponse) (err error) {
	height := len(req.CurrentWorld)-2
	width := len(req.CurrentWorld[0])
	res.WorldSlice = calculateNextState(height, width, req.CurrentWorld)
	return
}


func main() {
	pAddr := flag.String("port", "8082", "Port to listen on")
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