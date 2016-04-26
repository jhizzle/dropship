package main

import (
	"fmt"
	"github.com/klauspost/reedsolomon"
	"net"
	"os"
)

const fileName = "test.rnd"

const PktDataSize = 1200
const DataParts = 20
const ParityParts = 5

func main() {
	fmt.Printf("Hello DropShip!\n")
	f, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Couldn't open:", fileName)
		return
	}

	stat, err := f.Stat()
	if err != nil {
		fmt.Println("Couldnt' get stats on file:", fileName)
		return
	}

	fmt.Printf("Size: %d\n", stat.Size())

	data := make([][]byte, DataParts+ParityParts)

	for i := range data {
		data[i] = make([]byte, PktDataSize)
	}

	for i := 0; i < DataParts; i++ {
		n, err := f.Read(data[i])
		if n != PktDataSize {
			for j := 0; j < PktDataSize-n; j++ {
				data[i][n+j] = 0
			}
		}
		if err != nil {
			fmt.Printf("err: %s\n", err)
		}
	}

	encoder, err := reedsolomon.New(DataParts, ParityParts)
	if err != nil {
		fmt.Printf("Creating new encoder: %s\n", err)
		return
	}

	err = encoder.Encode(data)
	if err != nil {
		fmt.Printf("Encoding data: %s\n", err)
		return
	}

	for i := range data {
		fmt.Printf("%x\n", data[i])

	}

	udpDest, err := net.ResolveUDPAddr("udp", "localhost:1712")
	if err != nil {
		fmt.Printf("Couldn't resolve UDP address: %s\n", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, udpDest)
	if err != nil {
		fmt.Printf("Couldn't DialUDP: %s\n", err)
		return
	}

	n, err := conn.Write(data[0])
	fmt.Printf("Wrote: %d bytes, err: %s\n", n, err)

}
