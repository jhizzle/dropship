package main

import (
	"fmt"
	//    "io"
	//"bytes"
	"encoding/binary"
	//"errors"
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

// PktHeader generates a string representing the packet header to be sent with data.
func (s *Session) PktHeader() []byte {

	buf := make([]byte, 50) // arbitrary value large enough to hold all encoded bits
	sum := 0

	sum += binary.PutUvarint(buf, uint64(s.Id))
	sum += binary.PutUvarint(buf[sum:], uint64(s.Sequence))
	sum += binary.PutUvarint(buf[sum:], uint64(s.K))
	sum += binary.PutUvarint(buf[sum:], uint64(s.M))
	sum += binary.PutUvarint(buf[sum:], uint64(s.PktSize))

	return buf[:sum]
}

func DecodePktHeader(data []byte) (*Session, error) {
	var s Session

	offset := 0
	v, n := binary.Uvarint(data)
	s.Id = int(v)
	offset += n

	v, n = binary.Uvarint(data[offset:])
	s.Sequence = int(v)
	offset += n

	v, n = binary.Uvarint(data[offset:])
	s.K = int(v)
	offset += n

	v, n = binary.Uvarint(data[offset:])
	s.M = int(v)
	offset += n

	v, n = binary.Uvarint(data[offset:])
	s.PktSize = int(v)
	offset += n

	return &s, nil
}

// Send sends data with reed solomon encoded parity data to the destination
// specified in the Session. It returns the amount of data sent and nil error
// on success.
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

	header := s.PktHeader()

	for _, d := range parts {
		tmp := append(header, d...)
		n, err := s.Connection.Write(tmp)
		if n != len(tmp) {
			fmt.Printf("Couldn't send whole packet: %d of %d\n", n, s.PktSize+len(header))
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

// NewSession creates the information required for setting up a new DropShip
// session. The parameters k and m are used to specify the number of data parts
// and parity parts respectively. The parameter size determines how big each
// data and parity part should be. Dest is a string representation of the
// remote IP and port. See the "net" package for more details on the dest
// parameter. On success, NewSession will return a Session and nil error. On
// error, Session will be nil and the error will be set.
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

	fmt.Printf("Header: % x", s.PktHeader)
	msg := "Hello world! We're going to send a small message to test this out"

	n, err := s.Send([]byte(msg))
	fmt.Printf("Sent %d bytes of %d, err: %v\n", n, len(msg), err)

	fmt.Println(s)
}
