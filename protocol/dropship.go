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
	ErrCorrupt    = errors.New("hash failed")
	ArgumentError = errors.New("Argument out of bounds")
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

// DecodeMessage takes a slice of slice of bytes that form a message and
// decodes the values to create the original bytes.
func DecodeMessage(msg [][]byte) ([]byte, error) {

	for i := range msg {
		var valid bool
		data, valid := ValidateStripHash(msg[i])
		if !valid {
			msg[i] = nil
		} else {
			msg[i] = data
		}
	}

	var size int

	packets := make([]*Packet, K+M)
	for i := range msg {
		var p Packet
		if msg[i] != nil {
			err := proto.Unmarshal(msg[i], &p)
			if err == nil {
				packets[p.Seq] = &p
				size = int(p.Size)

			}
		}
	}

	msgData := make([][]byte, K+M)
	for i := range packets {
		if packets[i] != nil {
			msgData[i] = packets[i].Data
		}
	}

	enc, _ := reedsolomon.New(K, M)

	err := enc.Reconstruct(msgData)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)

	enc.Join(buf, msgData, size)

	return buf.Bytes()[:size], nil
}

type Message struct {
	Id        int
	K         int
	M         int
	Size      int
	Shards    [][]byte
	ShardSize int
	enc       reedsolomon.Encoder
}

func DataToMessage(data []byte, id, k, m, shardSize int) (*Message, error) {
	if id <= 0 || k <= 0 || m <= 0 || shardSize <= 0 {
		return nil, ArgumentError
	}

	msg := &Message{
		id,
		k,
		m,
		Min(len(data), k*shardSize),
		nil,
		shardSize,
		nil,
	}

	b := make([]byte, k*shardSize)
	copy(b, data)

	var err error
	msg.enc, err = reedsolomon.New(k, m)
	if err != nil {
		return nil, err
	}

	msg.Shards, err = msg.enc.Split(b)
	if err != nil {
		return nil, err
	}

	err = msg.enc.Encode(msg.Shards)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func DataFromMessage(m *Message) ([]byte, error) {
	var err error

	err = m.enc.Reconstruct(m.Shards)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = m.enc.Join(buf, m.Shards, m.Size)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func MessageToPackets(m *Message) ([]*Packet, error) {
	if m == nil {
		return nil, ArgumentError
	}

	packets := make([]*Packet, len(m.Shards))

	for i, shard := range m.Shards {
		p := &Packet{
			uint32(m.Id),
			uint32(i),
			uint32(m.K),
			uint32(m.M),
			uint32(m.Size),
			shard,
		}
		packets[i] = p
	}

	return packets, nil
}

func MessageFromPackets(packets []*Packet) (*Message, error) {
	var err error

	m := &Message{}

	// make sure packets are in order
	for i := range packets {
		for packets[i] != nil && packets[i].Seq != uint32(i) {
			seq := packets[i].Seq
			packets[i], packets[seq] = packets[seq], packets[i]
		}
	}

	// get message info
	for _, p := range packets {
		if p != nil {
			m.Id = int(p.Id)
			m.K = int(p.K)
			m.M = int(p.M)
			m.Size = int(p.Size)
			m.ShardSize = len(p.Data)
			m.enc, err = reedsolomon.New(m.K, m.M)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	// copy over data
	m.Shards = make([][]byte, m.K+m.M)
	for i, p := range packets {
		if p != nil {
			m.Shards[i] = p.Data
		}
	}

	// reconstruct data if any is missing
	err = m.enc.Reconstruct(m.Shards)
	if err != nil {
		return nil, err
	}

	return m, nil
}
