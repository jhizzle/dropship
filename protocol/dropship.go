package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/klauspost/reedsolomon"
	"hash/crc32"
	"io"
	"net"
	"time"
)

var _ = io.EOF
var _ = new(bytes.Buffer)
var _ = new(reedsolomon.Encoder)
var _ = new(net.UDPConn)
var _ = time.Second
var _ = proto.Marshal

const (
	K             = 20
	M             = 5
	PktSize       = 1200
	MsgSize       = K * PktSize
	MaxPacketSize = 1500
)

var (
	ErrCorrupt = errors.New("hash failed")
)

func main() {
	fmt.Printf("Yolo\n")
	s, _ := Dial("udp", ":1712")
	err := s.Close()
	fmt.Printf("Close Error: %v\n", err)
	fmt.Printf("LocalAddr: %s\n", s.LocalAddr())
	fmt.Printf("RemoteAddr: %s\n", s.RemoteAddr())
}

type Sender struct {
	remote *net.UDPAddr
	conn   net.Conn
	count  uint32
	enc    reedsolomon.Encoder
}

func Dial(network, address string) (*Sender, error) {
	switch network {
	case "udp":
	default:
		return nil, &net.OpError{
			Op:     "dial",
			Net:    network,
			Source: nil,
			Addr:   nil,
			Err:    net.UnknownNetworkError(network),
		}
	}

	var err error
	s := &Sender{}

	s.remote, err = net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: err}
	}

	s.conn, err = net.DialUDP(network, nil, s.remote)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: s.remote, Err: err}
	}

	s.enc, _ = reedsolomon.New(K, M)

	return s, nil
}

func (s *Sender) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func (s *Sender) Write(b []byte) (int, error) {
	total := 0

	for remaining := len(b); remaining > 0; {

		msg, n := s.MakeMessage(b[total:])
		remaining -= n

		for _, data := range msg {
			_, err := s.conn.Write(data)
			if err != nil {
				return total, err
			}
		}
		total += n

	}

	return total, nil
}

func (s *Sender) Close() error {
	return s.conn.Close()
}

func (s *Sender) LocalAddr() net.Addr {
	return s.conn.LocalAddr()

}

func (s *Sender) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Sender) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

func (s *Sender) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

func (s *Sender) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

type Receiver struct {
}

func (r *Receiver) Read(b []byte) (int, error) {
	return 0, nil
}

func (r *Receiver) Write(b []byte) (int, error) {
	return 0, nil
}

func (r *Receiver) Close() error {
	return nil
}

func (r *Receiver) LocalAddr() net.Addr {
	return &net.UDPAddr{}

}

func (r *Receiver) RemoteAddr() net.Addr {
	return &net.UDPAddr{}
}

func (r *Receiver) SetDeadline(t time.Time) error {
	return nil
}

func (r *Receiver) SetReadDeadline(t time.Time) error {
	return nil
}

func (r *Receiver) SetWriteDeadline(t time.Time) error {
	return nil
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

// SplitData takes an array of bytes and splits it into count slices, each of
// size size. It returns the split data (zero filled if needed) and the number
// of valid bytes in the returned data.
func SplitData(data []byte, size, count int) ([][]byte, int) {
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

// AppendHash appends a IEEE checksum to the data supplied.
func AppendHash(data []byte) []byte {
	hash := crc32.ChecksumIEEE(data)
	return append(data, []byte(fmt.Sprintf("%08x", hash))...)
}

// StripHash will remove a hash that was applied by AppendHash and return both
// the data that generated the hash and the hash in string form.
func StripHash(data []byte) ([]byte, string) {
	if len(data) < 8 {
		return data, ""
	}
	return data[:len(data)-8], string(data[len(data)-8:])
}

// ValidHash returns true if the hash is valid or false if it is not.
func ValidHash(data []byte, hash string) bool {
	calcHash := crc32.ChecksumIEEE(data)
	result := fmt.Sprintf("%08x", calcHash)
	if result != string(hash) {
		fmt.Printf("Expected: %s, Actual: %s\n", hash, result)
		return false
	}

	return true
}

// ValidateStripHash will validate the hash that is attached to the data and
// remove it if it's correct. It will return the unaltered data if the hash is
// not validated. It also returns true or false to indicate the validity of the
// hash.
func ValidateStripHash(data []byte) ([]byte, bool) {
	msg, hash := StripHash(data)
	valid := ValidHash(msg, hash)
	if !valid {
		return data, false
	}

	return msg, true
}

// MakeMessages creates a slice of byte slices that represent the encoded value
// of data. This data has been packetized and hashed for integrity. It returns
// a slice of byte slices where each slice is ready to be transmitted and the
// number of bytes that are actually encoded in the slice.
func (s *Sender) MakeMessage(data []byte) ([][]byte, int) {
	msg, n := SplitData(data, PktSize, K)

	for i := 0; i < M; i++ {
		msg = append(msg, make([]byte, PktSize))
	}

	s.enc.Encode(msg)

	packets := make([][]byte, K+M)
	for i := 0; i < K+M; i++ {
		p := &Packet{
			s.count,
			uint32(i),
			K,
			M,
			uint32(n),
			msg[i],
		}

		packets[i], _ = proto.Marshal(p)
		packets[i] = AppendHash(packets[i])
	}
	s.count++
	return packets, n
}

// PacketToWire converts a Packet into the
func PacketToWire(p *Packet) ([]byte, error) {
	data, err := proto.Marshal(p)
	if err != nil {
		return nil, err
	}

	data = AppendHash(data)
	return data, nil
}

func WireToPacket(data []byte) (*Packet, error) {
	data, valid := ValidateStripHash(data)
	if !valid {
		return nil, ErrCorrupt
	}

	p := new(Packet)
	err := proto.Unmarshal(data, p)
	if err != nil {
		return nil, err
	}

	return p, nil

}

//// DecodeMessage takes a slice of slice of bytes that form a message and
//// decodes the values to create the original bytes.
//func DecodeMessage(msg [][]byte) ([]byte, error) {
//
//	for i := range msg {
//		var valid bool
//		data, valid := ValidateStripHash(msg[i])
//		if !valid {
//			msg[i] = nil
//		} else {
//			msg[i] = data
//		}
//	}
//
//	packets := make([]*Packet, K+M)
//	for i := range msg {
//		var p Packet
//		if msg[i] != nil {
//			err := proto.Unmarshal(msg[i], &p)
//			if err == nil {
//				packets[i] = &p
//
//			}
//		}
//	}
//
//	msgData := make([][]byte, K+M)
//	for i := range packets {
//		msgData[i] = packets[i].Data
//	}
//
//	enc, _ := reedsolomon.New(K, M)
//
//	ok, _ := enc.Verify(msgData)
//	if !ok {
//		t.Errorf("Could not verify encoded values\n")
//	}
//
//	for i := 0; i < M; i++ {
//		msgData[rand.Intn(M)] = nil
//	}
//
//	err = enc.Reconstruct(msgData)
//	if err != nil {
//		t.Errorf("Failure to reconstruct data\n")
//	}
//
//	buf := new(bytes.Buffer)
//
//	enc.Join(buf, msgData, int(packets[0].Size))
//
//	if !bytes.Equal(buf.Bytes(), din[:K*PktSize]) {
//		t.Errorf("Sent data does not match received data.\n")
//	}
//
//}
