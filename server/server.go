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
	"uk.ac.bris.cs/gameoflife/util"
)

var turn int
var globaWorld [][]uint8
var globalAlive int
var mutex sync.Mutex
var paused = make(chan int)
var resume = make(chan int)

var turnChannel = make(chan int)
var flippedCellChannels = make(chan []util.Cell)
var inHaloChannel = make(chan []*stubs.HaloResponse)
var outHaloChannel = make(chan []*stubs.HaloResponse)

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
	//processGame = true
	globaWorld = makeMatrix(worldSize, worldSize)
	mutex.Unlock()
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


type GameOfLifeOperation struct{}

//need to send appropriate world slice to worker node or not
func workerNode(client *rpc.Client, startHeight, endHeight, width int, currentWorld [][]uint8, turns int, result chan [][]uint8){
	request := stubs.NodeRequest{Turns: turns, StartY: startHeight, EndY: endHeight, Width: width, CurrentWorld: currentWorld}
	response := new(stubs.NodeResponse)
	err := client.Call(stubs.ProcessSlice, request, response)
	if err != nil {
		fmt.Println("Could not call worker node")
	}
	result <- response.WorldSlice
}

func sendWorkers(req stubs.Request, workerConnections []*rpc.Client) [][]uint8 {

	workerHeight := req.ImageHeight / len(req.Workers)
	var newPixelData [][]uint8

	responses := make([]chan [][]uint8, len(req.Workers))
	for i := 0; i < len(req.Workers); i++ {
		responses[i] = make(chan [][]uint8)
	}

	for j := 0; j < len(req.Workers); j++ {
		startHeight := workerHeight*j
		endHeight :=  workerHeight*(j+1)
		if j == len(req.Workers) - 1 { // send the extra part when workerHeight is not a whole number in last iteration
			endHeight += req.ImageHeight % len(req.Workers)
		}
		relevantSlice := req.InitialWorld[startHeight:endHeight]
		go workerNode(workerConnections[j], startHeight, endHeight, req.ImageWidth, relevantSlice, req.Turns, responses[j])
	}

	for j := 0; j < len(req.Workers); j++ {
		result := <-responses[j]
		newPixelData = append(newPixelData, result...)
	}
	return newPixelData
}

func makeWorkerConnectionsAndChannels(workers []string) ([]*rpc.Client) {
	var clientConnections []*rpc.Client
	for i := 0; i < len(workers); i++ {
		client, errors := rpc.Dial("tcp", workers[i])
		if errors != nil{
			fmt.Println(errors)
		}
		clientConnections = append(clientConnections, client)
	}
	return clientConnections
}

func closeWorkerConnections(workerConnections []*rpc.Client){
	for _, client := range workerConnections {
		err := client.Close()
		if err != nil {
			fmt.Println(err)
		}
	}
}


func flipCellHandler(clients []*rpc.Client, turns int) {

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

// func to make return all top and bottom slices for all parts
func sendInitialHalo(req stubs.Request, clients []*rpc.Client){
	var halos []stubs.HaloResponse
	workerHeight := req.ImageHeight / len(req.Workers)
	for j := 0; j < len(req.Workers); j++ {
		var h1, h2 []uint8
		startHeight := workerHeight*j
		endHeight :=  workerHeight*(j+1)
		if j == len(req.Workers) - 1 { // send the extra part when workerHeight is not a whole number in last iteration
			endHeight += req.ImageHeight % len(req.Workers)
		}
		h1 = req.InitialWorld[startHeight]
		h2 = req.InitialWorld[endHeight-1]
		halos = append(halos, stubs.HaloResponse{FirstHalo: h1, LastHalo: h2})
	}
	for index, client := range clients{
		err := client.Call(stubs.SendHaloToNode, halos[index], &stubs.EmptyResponse{})
		if err != nil {
			return
		}
	}
}

//func to take all slices and arrange it to clients in order
func haloExchange(halos []stubs.HaloResponse) []stubs.HaloResponse {
	var newExchange []stubs.HaloResponse
	size := len(halos)-1

	if size == 0 {
		fmt.Printf("ONE CLIENT\n")
		newExchange = append(newExchange, stubs.HaloResponse{FirstHalo: halos[0].LastHalo, LastHalo: halos[0].FirstHalo})
		return newExchange
	}

	for i, _ := range halos {
		var h1, h2 []uint8
		if i == 0 {
			h1 = halos[size].LastHalo
			h2 = halos[i+1].FirstHalo
		} else if i == size {
			h1 = halos[size-1].FirstHalo
			h2 = halos[0].FirstHalo
		} else {
			h1 = halos[i-1].LastHalo
			h2 = halos[i+1].FirstHalo
		}
		newExchange = append(newExchange, stubs.HaloResponse{FirstHalo: h1, LastHalo: h2})
	}
	return newExchange
}



func getHalo(clients []*rpc.Client, turns int, req stubs.Request) {

	sendInitialHalo(req, clients)

	var haloResponses []*stubs.HaloResponse
	for i := 0; i < turns; i++ {
		response := new(stubs.HaloResponse)
		for _, client := range clients {
			err := client.Call(stubs.GetHaloRegions, stubs.EmptyRequest{}, response)
			if err != nil {
				//fmt.Println("GET HALO BROKEN")
				return
			}
			haloResponses = append(haloResponses, response)
		}
		//fmt.Println("\nGot all halos from all clients")
		go sendHalo(clients, turns)
		inHaloChannel <- haloResponses
		//fmt.Println("Passed all halos down channel")
	}
}


func dereference(ptr []*stubs.HaloResponse) []stubs.HaloResponse {
	var halos []stubs.HaloResponse
	for _, haloPointer := range ptr {
		halos = append(halos, *haloPointer)
	}
	return halos
}



func sendHalo(clients []*rpc.Client, turns int) {
		select {
		case sendback := <-inHaloChannel:
			//fmt.Println(dereference(sendback))
			//fmt.Println(dereference(sendback))
			halos := haloExchange(dereference(sendback))
			//fmt.Println("")//there is a nill array at first
			//fmt.Println(halos)
			//fmt.Println(halos)
			for index, client := range clients {
				//fmt.Println(halos[index])
				err := client.Call(stubs.SendHaloToNode,  halos[index], &stubs.EmptyResponse{})
				if err != nil {
					//fmt.Println("Couldn't not send halo back to node")
					return
				}
			}
		}
}


func getTurnsAndCellCount(clients []*rpc.Client, turns int) {

	for i := 0; i < turns; i++ {
		response := new(stubs.TurnResponse)
		var alive = 0
		for _, client := range clients{
			client.Call(stubs.GetTurnAndAliveCell, stubs.EmptyRequest{}, response)
			alive += response.NumOfAliveCells
			//fmt.Printf("alive from client %d - %d\n", index, response.NumOfAliveCells)
		}
		//fmt.Printf("%d,%d\n", turn, alive)
		mutex.Lock()
		globalAlive = alive
		turn = response.Turn
		turnChannel <- response.Turn
		mutex.Unlock()

	}
}




func (s *GameOfLifeOperation) CompleteTurn(req stubs.Request, res *stubs.Response) (err error) {
	if req.GameStatus == "NEW" {
		resetState(req.ImageWidth)
	}

	globaWorld = req.InitialWorld
	globalAlive = findAliveCellCount(globaWorld)

	workerConnections := makeWorkerConnectionsAndChannels(req.Workers)

	go flipCellHandler(workerConnections, req.Turns)
	go getTurnsAndCellCount(workerConnections, req.Turns)

	//go sendInitialHalo(req, workerConnections)

	go getHalo(workerConnections, req.Turns, req)

	final := sendWorkers(req, workerConnections)

	res.World = final // collect the world back together and return
	closeWorkerConnections(workerConnections)
	return
}

func (s *GameOfLifeOperation) AliveCellGetter(req stubs.EmptyRequest, res *stubs.TurnResponse) (err error) {
	mutex.Lock()
	res.Turn = turn
	res.NumOfAliveCells = globalAlive
	mutex.Unlock()
	return
}

func (s *GameOfLifeOperation) Shutdown(req stubs.EmptyRequest, res *stubs.EmptyResponse) (err error) {
	fmt.Println("Exiting...")
	//shutdown all the nodes aswell
	//processGame = false
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
	//processGame = false
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
		case turn := <- turnChannel:
			res.Turn = turn
		case flipped := <- flippedCellChannels:
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
