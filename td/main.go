package main

import (
	"bytes"
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
	"os"
	"time"
)

var _ = io.MultiWriter
var _ = rand.Int
var _ = os.Open

const (
	K             = 20
	M             = 5
	PktSize       = 1200
	MsgSize       = K * PktSize
	MaxPacketSize = 1500
)

type Shipper struct {
	conn  *net.UDPConn
	enc   reedsolomon.Encoder
	count uint32
}

func NewShipper(dest string) (*Shipper, error) {
	var err error
	s := &Shipper{}

	addr, err := net.ResolveUDPAddr("udp", dest)
	if err != nil {
		return nil, err
	}

	s.conn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}

	s.enc, err = reedsolomon.New(K, M)
	if err != nil {
		return nil, err
	}

	s.count = 1

	return s, nil
}

func (s *Shipper) Shutdown() {
	s.conn.Close()
}

func (s *Shipper) Send(data []byte) (int, error) {

	shards, consumed := splitData(data, PktSize, K)

	shards = append(shards, make([][]byte, M)...)

	for i := K; i < K+M; i++ {
		shards[i] = make([]byte, PktSize)
	}

	s.enc.Encode(shards)

	for i := 0; i < K+M; i++ {
		p := &Packet{
			s.count,
			uint32(i),
			0,
			K,
			M,
			uint32(consumed),
			shards[i],
		}

		pkt, _ := proto.Marshal(p)
		pkt = AddHash(pkt)
		_, err := s.conn.Write([]byte(pkt))
		if err != nil {
			return 0, err
		}
	}
	s.count++
	return consumed, nil
}

// Min finds the minimum value out of the array of integers.
func Min(a ...int) int {
	min := int(^uint(0) >> 1) // largest int
	for _, i := range a {
		if i < min {
			min = i
		}
	}
	return min
}

// splitData takes an array of bytes and splits it into count slices, each of
// size size. It returns the split data (zero filled if needed) and the number
// of usable data in the returned data.
func splitData(data []byte, size, count int) ([][]byte, int) {
	split := make([][]byte, count)

	var total int

	for i := 0; i < count; i++ {

		usable := Min(size, len(data))
		if usable < size {
			split[i] = make([]byte, size)
			copy(split[i], data)
		} else {
			split[i] = data[:size]
		}

		total += usable
		data = data[usable:]
	}

	return split, total
}

//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//

var (
	ERR_BAD_HASH = errors.New("Bad hash")
	ERR_TIMEOUT  = errors.New("Timed out")
)

var (
	isListener bool
)

func init() {
	flag.BoolVar(&isListener, "listen", false, "Use this flag to listen for a connection instead of sending data")
	flag.Parse()
}

type Hub struct {
	conn      *net.UDPConn
	enc       reedsolomon.Encoder
	receivers map[int]*Receiver
	routes    map[int]*bytes.Buffer
	eChans    map[int]chan error
}

func NewHub(bindAddr string) (*Hub, error) {
	var err error

	h := &Hub{}
	addr, err := net.ResolveUDPAddr("udp", bindAddr)
	if err != nil {
		return nil, err
	}

	h.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	h.enc, err = reedsolomon.New(K, M)
	if err != nil {
		return nil, err
	}

	h.routes = make(map[int]*bytes.Buffer)
	h.eChans = make(map[int]chan error)
	h.receivers = make(map[int]*Receiver)
	return h, nil
}

func (h *Hub) RegisterReceiver(route int, w io.Writer) chan error {
	buf := new(bytes.Buffer)
	h.routes[route] = buf
	go buf.WriteTo(w)

	h.eChans[route] = make(chan error)

	return h.eChans[route]
}

type Receiver struct {
	Route int
	Data  bytes.Buffer
	Err   chan error
}

func (h *Hub) NewReceiver(route int) *Receiver {
	r := &Receiver{route, bytes.Buffer{}, make(chan error)}
	h.receivers[route] = r
	return r
}

func (r *Receiver) Read(p []byte) (int, error) {
	return r.Data.Read(p)
}

func (r *Hub) Shutdown() {
	r.conn.Close()

}

func AddHash(data []byte) []byte {
	hash := crc32.ChecksumIEEE(data)
	return append(data, []byte(fmt.Sprintf("%08x", hash))...)
}

func StripHash(data []byte) ([]byte, error) {
	if len(data) < 8 {
		return nil, ERR_BAD_HASH
	}
	data, hashStr := data[:len(data)-8], data[len(data)-8:]
	hash := crc32.ChecksumIEEE(data)
	result := fmt.Sprintf("%08x", hash)
	if result != string(hashStr) {
		fmt.Printf("Expected: %s, Actual: %s\n", string(hashStr), result)
		return data, ERR_BAD_HASH
	}

	return data, nil
}

func (h *Hub) Recv(quit chan bool) {

	for {
		select {
		case <-quit:
			fmt.Println("Quiting the receiver\n")
			break

		default:
			p, err := h.RecvPacket()
			// TODO: handle errors and send them
			if err != nil {
				log.Fatal("TODO: handle errors in Hub.Recv\n")
			}
			// always succeeds because bytes.Buffer panics if there's no memory
			fmt.Printf("Writing %d bytes\n", len(p.Data))
			//h.routes[int(p.Route)].Write(p.Data)
			h.receivers[int(p.Route)].Data.Write(p.Data)
		}
	}
}

func (r *Hub) RecvMessage(c chan *Packet) ([]byte, error) {
	shards := make([][]byte, K+M)
	var size int

	for i := 0; i < K; i++ {
		select {
		case p := <-c:
			shards[p.Seq] = p.Data
			size = int(p.Size)

		case <-time.After(10 * time.Second):
			return nil, ERR_TIMEOUT

		}
	}

	r.enc.Reconstruct(shards)

	buf := &bytes.Buffer{}
	err := r.enc.Join(buf, shards, size)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (r *Hub) RecvPacket() (*Packet, error) {
	buf := make([]byte, MaxPacketSize)

	deadline := time.Now()
	deadline = deadline.Add(time.Second)
	r.conn.SetReadDeadline(deadline)
	n, err := r.conn.Read(buf)
	if err != nil {
		return nil, err
	}

	buf, err = StripHash(buf[:n])
	if err != nil {
		return nil, err
	}

	p := &Packet{}

	err = proto.Unmarshal(buf, p)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func main() {

	h, err := NewHub(":1712")
	if err != nil {
		log.Fatalf("Creating new receiver failed: %s\n", err)
	}
	defer h.Shutdown()

	r := h.NewReceiver(0)
	fmt.Println("r", r)

	quit := make(chan bool)
	go h.Recv(quit)

	time.Sleep(time.Second)

	shipper, err := NewShipper("localhost:1712")
	if err != nil {
		log.Fatal("Couldn't create new shipper: %s\n", err)
	}
	defer shipper.Shutdown()

	n, err := shipper.Send([]byte("Hello world!"))
	fmt.Printf("Sent %d bytes, error: %v\n", n, err)

	go func() {
		buf := make([]byte, 1000)

		n, err := r.Read(buf)
		fmt.Printf("Read %d bytes: '%s', error: %v\n", n, buf, err)
	}()

	time.Sleep(time.Second)

	fmt.Println("Trying to quit\n")
	quit <- true
}
