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

func findAliveCellCount( world [][]uint8) int {
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
func workerNode(client *rpc.Client, startHeight, endHeight, width int, currentWorld [][]uint8, turns int){
	request := stubs.NodeRequest{Turns: turns, StartY: startHeight, EndY: endHeight, Width: width, CurrentWorld: currentWorld}
	response := new(stubs.NodeResponse)
	err := client.Call(stubs.ProcessSlice, request, response)
	if err != nil {
		fmt.Println("workerNode")
	}
}

func sendWorkers(req stubs.Request, workerConnections []*rpc.Client)  {

	workerHeight := req.ImageHeight / len(req.Workers)

	for j := 0; j < len(req.Workers); j++ {
		startHeight := workerHeight*j
		endHeight :=  workerHeight*(j+1)
		if j == len(req.Workers) - 1 { // send the extra part when workerHeight is not a whole number in last iteration
			endHeight += req.ImageHeight % len(req.Workers)
		}
		fmt.Println(startHeight, endHeight)
		//s := globaWorld[startHeight:endHeight]
		workerNode(workerConnections[j], startHeight, endHeight, req.ImageWidth, req.InitialWorld, req.Turns)
	}
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

func aliveCellHandler(clients []*rpc.Client, turns int) {

	for i := 0; i < turns; i++ {
		var alive = 0
		for _, client := range clients{
			response := new(stubs.AliveCellCountResponse)
			client.Call(stubs.GetAliveCellCount, stubs.EmptyRequest{}, response)
			//fmt.Println("alive cells - "+ strconv.Itoa(response.Count))
			alive += response.Count
		}
		mutex.Lock()
		globalAlive = alive
		mutex.Unlock()

	}
}

func UpdateTurns(clients []*rpc.Client, turns int) {

	for i := 0; i < turns; i++ {
		response := new(stubs.TurnResponse)
		for _, client := range clients{
			client.Call(stubs.GetTurn, stubs.EmptyRequest{}, response)
			//fmt.Println("turns  - "+ strconv.Itoa(response.Turn))
		}
		mutex.Lock()
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
	go aliveCellHandler(workerConnections, req.Turns)
	go UpdateTurns(workerConnections, req.Turns)

	sendWorkers(req, workerConnections)


	select {
	case <-paused:
		<-resume
	default:
		break
	}


	res.World = globaWorld // function eh to get last world, same as GetWorld
	closeWorkerConnections(workerConnections)
	return
}

func (s *GameOfLifeOperation) GetAliveCell(req stubs.EmptyRequest, res *stubs.TurnResponse) (err error) {
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
	pAddr := flag.String("port", "8000", "Port to listen on")
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
