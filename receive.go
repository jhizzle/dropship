package main

import (
	"fmt"
	"net"
)

func main() {
	fmt.Printf("hello receiver\n")

	udpListen, err := net.ResolveUDPAddr("udp", ":1712")
	if err != nil {
		fmt.Printf("Error resolving UDP Addr: %s\n", err)
	}

	conn, err := net.ListenUDP("udp", udpListen)
	if err != nil {
		fmt.Println("Error listening: %s\n", err)
		return
	}

	buf := make([]byte, 3000)
	n, addr, err := conn.ReadFrom(buf)
	fmt.Printf("Read %d bytes from %v\n", n, addr)
	if err != nil {
		fmt.Println("ReadFromUDP: %s\n", err)
		return
	}

	fmt.Printf("Read: %d bytes from %v\n\n", n, addr)
	fmt.Printf("Data: %x\n", buf[:n])
}
