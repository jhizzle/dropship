package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/klauspost/reedsolomon"
	"hash/crc32"
	"io"
	"log"
	"math/rand"
	"net"
)

var _ = io.MultiWriter

const (
	K             = 20
	M             = 5
	PktSize       = 1200
	MsgSize       = K * PktSize
	MaxPacketSize = 1500
)

var (
	ERR_BAD_HASH = errors.New("Bad hash")
)

var (
	isListener bool
)

func init() {
	flag.BoolVar(&isListener, "listen", false, "Use this flag to listen for a connection instead of sending data")
	flag.Parse()
}

type Sender struct {
	Dest    *net.UDPAddr
	Conn    *net.UDPConn
	Encoder reedsolomon.Encoder
	Buffer  [][]byte
	Msg     uint32
}

type Receiver struct {
	LAddr   *net.UDPAddr
	Conn    *net.UDPConn
	Encoder reedsolomon.Encoder
	Buffer  [][]byte
	Msg     uint32
}

func NewSender(dest string) (*Sender, error) {
	var err error
	s := &Sender{}
	s.Dest, err = net.ResolveUDPAddr("udp", dest)
	if err != nil {
		return nil, err
	}

	s.Conn, err = net.DialUDP("udp", nil, s.Dest)
	if err != nil {
		return nil, err
	}

	s.Encoder, err = reedsolomon.New(K, M)
	if err != nil {
		return nil, err
	}

	s.Buffer = make([][]byte, K+M)
	for i := range s.Buffer {
		s.Buffer[i] = make([]byte, PktSize)
	}

	return s, nil
}

func Min(a ...int) int {
	min := int(^uint(0) >> 1) // largest int
	for _, i := range a {
		if i < min {
			min = i
		}
	}
	return min
}

func (s *Sender) Send(data []byte) (int, error) {
	dataLen := len(data)

	offset := 0
	size := 0
	for i := 0; i < K; i++ {
		amount := Min(MsgSize-size, PktSize, dataLen)
		if amount < PktSize {
			s.Buffer[i] = make([]byte, PktSize)
			if amount > 0 {
				copy(s.Buffer[i], data[offset:offset+amount])
			}
		} else {
			s.Buffer[i] = data[offset : offset+PktSize]
		}
		offset += amount
		size += amount
	}

	err := s.Encoder.Encode(s.Buffer)
	if err != nil {
		return 0, err
	}

	for i := range s.Buffer {
		p := &Packet{
			0,
			s.Msg,
			uint32(i),
			K,
			M,
			uint32(size),
			s.Buffer[i],
		}

		wire_data, err := proto.Marshal(p)
		if err != nil {
			return 0, err
		}

		wire_data = AddHash(wire_data)

		sent, err := s.Conn.Write(wire_data)
		if err != nil {
			return 0, err
		}
		if sent != len(wire_data) {
			panic("Couldn't send whole UDP Packet")
		}
	}
	return size, nil
}

func NewReceiver(bindAddr string) (*Receiver, error) {
	var err error
	r := &Receiver{}
	r.LAddr, err = net.ResolveUDPAddr("udp", bindAddr)
	if err != nil {
		return nil, err
	}

	r.Conn, err = net.ListenUDP("udp", r.LAddr)
	if err != nil {
		return nil, err
	}

	r.Buffer = make([][]byte, K+M)

	r.Encoder, err = reedsolomon.New(K, M)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func AddHash(data []byte) []byte {
	hash := crc32.ChecksumIEEE(data)
	return append(data, []byte(fmt.Sprintf("%08x", hash))...)
}

func StripHash(data []byte) ([]byte, error) {
	data, hashStr := data[:len(data)-8], data[len(data)-8:]
	hash := crc32.ChecksumIEEE(data)
	result := fmt.Sprintf("%08x", hash)
	if result != string(hashStr) {
		return data, ERR_BAD_HASH
	}

	return data, nil
}

func (r *Receiver) Recv(data []byte) (int, error) {
	buf := make([]byte, MaxPacketSize)
	n, raddr, err := r.Conn.ReadFrom(buf)
	fmt.Printf("Read: %d bytes from %v\n", n, raddr)
	if err != nil {
		return 0, err
	}

	buf, err = StripHash(buf)
	if err != nil {
		return 0, err
	}

	p := &Packet{}

	err = proto.Unmarshal(buf, p)
	if err != nil {
		return 0, err
	}

	fmt.Printf("Recieved this! %v\n", p)

	return len(buf), nil
}

//func Listen(laddr string) error {
//
//	for i := 0; i < K+M; i++ {
//		n, raddr, err := .ReadFrom(buf)
//		if err != nil {
//			return err
//		}
//		fmt.Printf("Read: %d bytes from %v\n", n, raddr)
//	}
//
//}

func (r *Receiver) ReceiveMessages(w io.Writer) error {
	buf := make([]byte, 1500)

	for {
		n, err := r.Recv(buf)
	}

}

func main() {

	if isListener {
		fmt.Printf("I'm a listener!\n")
		r, err := NewReceiver(":1712")
		if err != nil {
			log.Fatal("Creating new receiver failed")
		}
		buf := make([]byte, 1500)
		n, err := r.Recv(buf)
		fmt.Printf("Received: %d bytes.\n", n)
		return
	}

	p := &Packet{
		uint32(rand.Int()),
		uint32(rand.Int()),
		uint32(rand.Int()),
		uint32(rand.Int()),
		uint32(rand.Int()),
		uint32(rand.Int()),
		make([]byte, 2),
	}

	b := &Packet{
		0xffffffff,
		0xffffffff,
		0xffffffff,
		0xffffffff,
		0xffffffff,
		0xffffffff,
		make([]byte, 1),
	}

	fmt.Printf("Smallest size: %d\n", proto.Size(&Packet{}))
	fmt.Printf("Packet: %v\n\n", &Packet{})

	fmt.Printf("Largest size: %d\n", proto.Size(b))
	fmt.Printf("Packet: %v\n\n", b)

	fmt.Printf("Random Size: %d\n", proto.Size(p))
	fmt.Printf("Packet: %v\n\n", p)

	data, err := proto.Marshal(p)
	if err != nil {
		log.Fatal("marshalling error: ", err)
	}

	data = AddHash(data)

	fmt.Printf("Marshalled data is %d bytes\n", len(data))

	r := &Packet{}

	if err := proto.Unmarshal(data[:len(data)-8], r); err != nil {
		log.Fatalln("Failed to parse Packet:", err)
	}
	fmt.Printf("Readback: %v\n", r)
	fmt.Printf("Hash attached: %s\n", data[len(data)-8:])
	fmt.Printf("Hash Calculated: %08x\n", crc32.ChecksumIEEE(data[:len(data)-8]))

}
