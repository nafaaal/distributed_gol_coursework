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

func resetState(worldSize int){
	mutex.Lock()
	turn = 0
	processGame = true
	world = makeMatrix(worldSize, worldSize)
	mutex.Unlock()
}


type GameOfLifeOperation struct{}

func workerNode(client *rpc.Client, startHeight, endHeight, width int, currentWorld [][]uint8, results chan [][]uint8){
	request := stubs.NodeRequest{StartY: startHeight, EndY: endHeight, Width: width, CurrentWorld: currentWorld}
	response := new(stubs.NodeResponse)
	err := client.Call(stubs.ProcessSlice, request, response)
	if err != nil {
		fmt.Println(err)
		fmt.Println("workerNode")
	}
	results <- response.WorldSlice
}

func getNextWorld(req stubs.Request, workerConnections []*rpc.Client, workerChannels []chan [][]uint8) [][]uint8 {
	var newPixelData [][]uint8
	workerHeight := req.ImageHeight / len(req.Workers)

	for j := 0; j < len(req.Workers); j++ {
		startHeight := workerHeight*j
		endHeight :=  workerHeight*(j+1)
		if j == len(req.Workers) - 1 { // send the extra part when workerHeight is not a whole number in last iteration
			endHeight += req.ImageHeight % len(req.Workers)
		}
		go workerNode(workerConnections[j], startHeight, endHeight, req.ImageWidth, world, workerChannels[j])
	}

	for k := 0; k < len(req.Workers); k++ {
		result := <- workerChannels[k]
		newPixelData = append(newPixelData, result...)
	}
	return newPixelData
}

func makeWorkerConnectionsAndChannels(workers []string) ([]*rpc.Client, []chan [][]uint8) {
	var clientConnections []*rpc.Client
	for i := 0; i < len(workers); i++ {
		client, _ := rpc.Dial("tcp", workers[i]+":8081")
		clientConnections = append(clientConnections, client)
	}

	workerChannels := make([]chan [][]uint8, len(workers))
	for i := 0; i < len(workers); i++ {
		workerChannels[i] = make(chan [][]uint8)
	}

	return clientConnections, workerChannels
}

func closeWorkerConnections(workerConnections []*rpc.Client){
	for _, client := range workerConnections {
		err := client.Close()
		if err != nil {
			fmt.Println(err)
		}
	}
}


func (s *GameOfLifeOperation) CompleteTurn(req stubs.Request, res *stubs.Response) (err error) {
	if req.GameStatus == "NEW" {
		resetState(req.ImageWidth)
	}

	world = req.InitialWorld

	workerConnections, workerChannels := makeWorkerConnectionsAndChannels(req.Workers)

	for turn < req.Turns && processGame {

		newWorld := getNextWorld(req, workerConnections, workerChannels)

		mutex.Lock()
		world = newWorld
		turn++
		worldChannel <- world
		turnChannel <- turn
		mutex.Unlock()

		select {
		case <-paused:
			<-resume
		default:
			break
		}

	}
	res.World = world
	closeWorkerConnections(workerConnections)
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
	//shutdown all the nodes aswell
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


func (s *GameOfLifeOperation) ResetState(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error) {
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
