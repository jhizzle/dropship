package main

import (
	"fmt"
	//    "io"
	"github.com/klauspost/reedsolomon"
	"math/rand"
	"net"
)

type Session struct {
	Id          int
	Sequence    int
	K           int
	M           int
	PktSize     int
	Destination *net.UDPAddr
	Connection  *net.UDPConn
	Encoder     reedsolomon.Encoder
}

func (s *Session) String() string {
	return fmt.Sprintf("ID: %x, Sequence: %d, K: %d, M: %d, PktSize: %d, Destination: %s",
		s.Id,
		s.Sequence,
		s.K,
		s.M,
		s.PktSize,
		s.Destination,
	)
}

func (s *Session) Send(data []byte) (int, error) {
	size := len(data)
	burst_size := (s.K + s.M) * s.PktSize
	if size > burst_size {
		size = burst_size
	}

	// It's easier to just allocate a full buffer...
	var buf []byte
	if size < burst_size {
		buf = make([]byte, burst_size)
		copy(buf, data)
	} else {
		buf = data[:size]
	}

	parts := make([][]byte, s.K+s.M)
	for i := 0; i < s.K; i++ {
		parts[i] = buf[i*s.PktSize : (i+1)*s.PktSize]
	}

	for i := 0; i < s.M; i++ {
		parts[s.K+i] = make([]byte, s.PktSize)
	}

	err := s.Encoder.Encode(parts)
	if err != nil {
		return 0, err
	}

	str := fmt.Sprintf("%#x,%d,%d,%d,", s.Id, s.K, s.M, s.PktSize)
	var header []byte
	header = append(header, []byte(str)...)

	for _, d := range parts {
		seq := fmt.Sprintf("%#016x.", s.Sequence)
		tmp := append(header, seq...)
		tmp = append(tmp, d...)
		n, err := s.Connection.Write(tmp)
		if n != len(tmp) {
			fmt.Printf("Couldn't send whole packet: %d of %d\n", n, s.PktSize+len(str))
			return 0, err
		}
		if err != nil {
			fmt.Printf("Couldn't write packet because: %s\n", err)
			return 0, err
		}
		s.Sequence = s.Sequence + 1
	}

	return size, nil
}

func NewSession(k, m, size int, dest string) (*Session, error) {
	addr, err := net.ResolveUDPAddr("udp", dest)
	if err != nil {
		return nil, err
	}

	encoder, err := reedsolomon.New(k, m)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}

	return &Session{rand.Int(), 0, k, m, size, addr, conn, encoder}, nil
}

//func SendFile(filename, destination string) {
//}
//
//func SendHeader(filename, destination string) {
//}
//
//func SendData(data []byte, destination string) {
//}

func main() {
	fmt.Printf("Hello\n")

	s, err := NewSession(20, 5, 1200, "localhost:1712")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	msg := "Hello world! We're going to send a small message to test this out"

	n, err := s.Send([]byte(msg))
	fmt.Printf("Sent %d bytes of %d, err: %v\n", n, len(msg), err)

	fmt.Println(s)
}
