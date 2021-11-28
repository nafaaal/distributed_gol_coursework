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

var turn int
var globaWorld [][]uint8
var globalAlive int
var mutex sync.Mutex
var clients []*rpc.Client

var turnChannel = make(chan int)
var flippedCellChannels = make(chan []util.Cell)

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

type GameOfLifeOperation struct{}

func workerNode(client *rpc.Client, startHeight, endHeight, width int, currentWorld [][]uint8, turns int, result chan [][]uint8) {
	request := stubs.NodeRequest{Turns: turns, StartY: startHeight, EndY: endHeight, Width: width, CurrentWorld: currentWorld}
	response := new(stubs.NodeResponse)
	err := client.Call(stubs.ProcessSlice, request, response)
	if err != nil {
		fmt.Println("Could not call worker node")
	}
	result <- response.WorldSlice
}

func sendWorkers(req stubs.Request) [][]uint8 {

	workerHeight := req.ImageHeight / len(req.Workers)
	var newPixelData [][]uint8

	responses := make([]chan [][]uint8, len(req.Workers))
	for i := 0; i < len(req.Workers); i++ {
		responses[i] = make(chan [][]uint8)
	}

	for j := 0; j < len(req.Workers); j++ {
		startHeight := workerHeight * j
		endHeight := workerHeight * (j + 1)
		if j == len(req.Workers)-1 { // send the extra part when workerHeight is not a whole number in last iteration
			endHeight += req.ImageHeight % len(req.Workers)
		}

		relevantSlice := req.InitialWorld[startHeight:endHeight]

		go workerNode(clients[j], startHeight, endHeight, req.ImageWidth, relevantSlice, req.Turns, responses[j])
	}

	for j := 0; j < len(req.Workers); j++ {
		result := <-responses[j]
		newPixelData = append(newPixelData, result...)
	}
	return newPixelData
}

func makeWorkerConnectionsAndChannels(workers []string) []*rpc.Client {
	var clientConnections []*rpc.Client
	for i := 0; i < len(workers); i++ {
		client, errors := rpc.Dial("tcp", workers[i])
		if errors != nil {
			fmt.Println(errors)
		}
		clientConnections = append(clientConnections, client)
	}
	return clientConnections
}

func closeWorkerConnections() {
	for _, client := range clients {
		err := client.Close()
		if err != nil {
			fmt.Println(err)
		}
	}
}

func flipCellHandler(turns int) {
	for i := 0; i < turns; i++ {
		var flippedCell []util.Cell
		for _, client := range clients {
			response := new(stubs.FlippedCellResponse)
			client.Call(stubs.GetFlippedCells, stubs.EmptyRequest{}, response)
			flippedCell = append(flippedCell, response.FlippedCells...)
		}
		flippedCellChannels <- flippedCell
	}
}

// func to make return all top and bottom slices for all parts 512 0, 511   0 255 256 511
func sendInitialHalo(req stubs.Request) {
	var halos []stubs.HaloResponse
	workerHeight := req.ImageHeight / len(req.Workers)
	for j := 0; j < len(req.Workers); j++ {
		var h1, h2 []uint8
		startHeight := workerHeight * j
		endHeight := workerHeight * (j + 1)
		if j == len(req.Workers)-1 { // send the extra part when workerHeight is not a whole number in last iteration
			endHeight += req.ImageHeight % len(req.Workers)
		}
		h1 = req.InitialWorld[startHeight]
		h2 = req.InitialWorld[endHeight-1]
		halos = append(halos, stubs.HaloResponse{FirstHalo: h1, LastHalo: h2})
	}
	sendHalo(halos)

}

//func to take all slices and arrange it to clients in order
func haloExchange(oldHalos []stubs.HaloResponse) []stubs.HaloResponse { //if 2 stubs.Haloresponses
	var newHalos []stubs.HaloResponse
	size := len(oldHalos) - 1
	if size == 0 {
		newHalos = append(newHalos, stubs.HaloResponse{FirstHalo: oldHalos[0].LastHalo, LastHalo: oldHalos[0].FirstHalo})
	} else {
		for i := range oldHalos {
			var halo stubs.HaloResponse
			if i == 0 {
				halo = stubs.HaloResponse{FirstHalo:  oldHalos[size].LastHalo, LastHalo: oldHalos[i+1].FirstHalo}
			} else if i == size {
				halo = stubs.HaloResponse{FirstHalo:  oldHalos[size-1].LastHalo, LastHalo: oldHalos[0].FirstHalo}
			} else {
				halo = stubs.HaloResponse{FirstHalo:  oldHalos[i-1].LastHalo, LastHalo: oldHalos[i+1].FirstHalo}
			}
			newHalos = append(newHalos, halo)
		}
	}
	return newHalos
}

func sendHalo(halos []stubs.HaloResponse) {
	halo := haloExchange(halos)
	for index, client := range clients {
		var hell = halo[index]
		err := client.Call(stubs.SendHaloToNode, hell, &stubs.EmptyResponse{})
		if err != nil {
			return
		}
	}
}

// }

func haloWorker(turns int, req stubs.Request) {

	sendInitialHalo(req)
	for i := 0; i < turns; i++ {
		var haloResponses []stubs.HaloResponse
		for _, client := range clients {
			response := new(stubs.HaloResponse)
			err := client.Call(stubs.SendHaloToBroker, stubs.EmptyRequest{}, response)
			if err != nil {
				return
			}
			haloResponses = append(haloResponses, *response)
		}
		sendHalo(haloResponses)
	}
}

func getTurnsAndCellCount(turns int) {

	for i := 0; i < turns; i++ {
		response := new(stubs.TurnResponse)
		var alive = 0
		for _, client := range clients {
			client.Call(stubs.GetTurnAndAliveCell, stubs.EmptyRequest{}, response)
			alive += response.NumOfAliveCells
		}
		mutex.Lock()
		globalAlive = alive
		turn = response.Turn
		turnChannel <- response.Turn
		mutex.Unlock()

	}
}

func (s *GameOfLifeOperation) CompleteTurn(req stubs.Request, res *stubs.Response) (err error) {

	globaWorld = req.InitialWorld
	globalAlive = findAliveCellCount(globaWorld)

	clients = makeWorkerConnectionsAndChannels(req.Workers)

	go flipCellHandler(req.Turns)
	go getTurnsAndCellCount(req.Turns)
	go haloWorker(req.Turns, req)

	final := sendWorkers(req)

	res.World = final
	closeWorkerConnections()
	return
}

func (s *GameOfLifeOperation) AliveCellGetter(req stubs.EmptyRequest, res *stubs.TurnResponse) (err error) {
	mutex.Lock()
	res.Turn = turn
	res.NumOfAliveCells = globalAlive
	mutex.Unlock()
	return
}

func (s *GameOfLifeOperation) GetWorld(req stubs.EmptyRequest, res *stubs.WorldResponse) (err error) {
	mutex.Lock()
	res.World = globaWorld //make a function to call all nodes and get their slices and make into 1
	mutex.Unlock()
	return
}

//GetWorldPerTurn FUNCTION NEED TO CHANGE
func (s *GameOfLifeOperation) GetWorldPerTurn(req stubs.EmptyRequest, res *stubs.SdlResponse) (err error) {
	for i := 0; i < 2; i++ {
		select {
		case turn := <-turnChannel:
			res.Turn = turn
		case flipped := <-flippedCellChannels:
			res.FlippedCells = flipped
		}
	}
	return
}

func main() {
	pAddr := flag.String("port", "8003", "Port to listen on")
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
