package main

import (
	"bytes"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/klauspost/reedsolomon"
	"hash/crc32"
	"io"
	"math/rand"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestDial(t *testing.T) {
	var err error

	_, err = Dial("udp", "127.0.0.1:1712")
	if err != nil {
		t.Errorf("Dial Failed: %s\n", err)
	}

	_, err = Dial("tcp", "127.0.0.1:1712")
	if err == nil {
		t.Errorf("Dial accepted TCP\n")
	}

}

func TestSenderRead(t *testing.T) {
	c, err := Dial("udp", "127.0.0.1:1712")
	if err != nil {
		t.Errorf("Dial Failed: %s\n", err)
	}

	data := make([]byte, 256)
	n, err := c.Read(data)

	if (n != 0) || (err != io.EOF) {
		t.Errorf("Expected: (0, io.EOF), Actual: (%d, %v)\n", n, err)
	}

}

func TestSenderWrite(t *testing.T) {
	s, err := Dial("udp", "127.0.0.1:1712")
	if err != nil {
		t.Errorf("Dial Failed: %v\n", err)
	}

	laddr, err := net.ResolveUDPAddr("udp", ":1712")
	if err != nil {
		t.Errorf("ResolveUDPAddr failed to resolve address: %s\n", err)
	}
	r, err := net.ListenUDP("udp", laddr)
	if err != nil {
		t.Errorf("Failed to listen on udp: %s\n", err)
	}

	msg := []byte("Hello DropShip!")
	n, err := s.Write(msg)
	if n != len(msg) || err != nil {
		t.Errorf("Write:: Expected: (%d, nil), Actual: (%d, %v)\n", len(msg), n, err)
	}

	pktData := make([]byte, PktSize)
	copy(pktData, msg)

	p := &Packet{
		27,
		2,
		K,
		M,
		uint32(len(msg)),
		pktData,
	}

	expect, _ := proto.Marshal(p)

	buf := make([]byte, MaxPacketSize)
	r.SetDeadline(time.Now().Add(time.Second))
	n, err = r.Read(buf[:len(expect)])
	if n != len(expect) || err != nil {
		t.Errorf("Expected: (%d, nil), Actual: (%d, %v)\n", len(expect), n, err)
	}
}

func TestAppendVerifyRemoveHash(t *testing.T) {
	buf := make([]byte, 1000)
	hash := crc32.ChecksumIEEE(buf)
	hashStr := fmt.Sprintf("%08x", hash)
	data := append(buf, []byte(hashStr)...)

	resultData, valid := ValidateStripHash(data)
	if !bytes.Equal(buf, resultData) || !valid {
		t.Errorf("Expected: (len(resultData) == %d, true), Actual: (%d, %v)\n", len(buf), len(resultData), valid)
	}

	copy(buf, []byte("Super Duper!"))

	data = AppendHash(buf)
	resultData, valid = ValidateStripHash(data)
	if !bytes.Equal(buf, resultData) || !valid {
		t.Errorf("Expected: (len(resultData) == %d, true), Actual: (%d, %v)\n", len(buf), len(resultData), valid)
	}

}

func TestSplitData(t *testing.T) {
	msg := []byte("Hello World\n")

	splitSize := 100
	splitCount := 10

	splits, n := SplitData(msg, splitSize, splitCount)
	if splits == nil {
		t.Errorf("splits is null\n")
	}
	if n != len(msg) {
		t.Errorf("didn't consume all data when it should have\n")
	}

	for i := 0; i < splitCount; i++ {
		if splits[i] == nil {
			t.Errorf("splits[%d] is nil\n", i)
		}
		if len(splits[i]) != splitSize {
			t.Errorf("splits[%d] is wrong length. Expected: %d, Actual: %d\n", i, splitCount, len(splits[i]))
		}
	}

	zeros := make([]byte, splitSize)
	if !bytes.Equal(msg, splits[0][:len(msg)]) {
		t.Errorf("Split does not contain correct data\n")
	}
	if !bytes.Equal(splits[0][len(msg):], zeros[len(msg):]) {
		t.Errorf("Split does contain zeroed data\n")
	}

	for i := 1; i < splitCount; i++ {
		if !bytes.Equal(splits[i], zeros) {
			t.Errorf("splits[%d] does contain zeroed data\n", i)
		}

	}

	splitSize = 300
	splitCount = 7

	msg = make([]byte, 5000)
	for i := range msg {
		msg[i] = byte(i)
	}

	splits, n = SplitData(msg, splitSize, splitCount)
	if splits == nil {
		t.Errorf("splits is null\n")
	}
	if n != splitSize*splitCount {
		t.Errorf("Consumed %d bytes when expected to consume %d bytes\n", n, splitSize*splitCount)
	}

	for i := 0; i < splitCount; i++ {
		if splits[i] == nil {
			t.Errorf("splits[%d] is nil\n", i)
		}
		if len(splits[i]) != splitSize {
			t.Errorf("splits[%d] is wrong length. Expected: %d, Actual: %d\n", i, splitCount, len(splits[i]))
		}
	}

	for i := 0; i < splitCount*splitSize; i++ {
		index := i / splitSize
		offset := i % splitSize
		if splits[index][offset] != byte(i) {
			t.Errorf("Split doesn't contain correct data: split %d, byte %d, expected value: %d, actual value: %d\n",
				index,
				byte(i),
				i,
				splits[index][offset],
			)
		}
	}

}

func TestMakeMessage(t *testing.T) {
	dinSize := 32000
	din := make([]byte, dinSize)
	rand.Read(din)

	s, err := Dial("udp", "127.0.0.1:1712")
	if err != nil {
		t.Errorf("Dial Failed: %v\n", err)
	}

	msg, n := s.MakeMessage(din)
	if n != K*PktSize {
		t.Errorf("Didn't use maximum data. Expected: %d, Actual: %d\n", K*PktSize, n)
	}

	if len(msg) != K+M {
		t.Errorf("Didn't produce K+M packets. Expected: %d, Actual: %d\n", K+M, len(msg))
	}

	dout := make([][]byte, K+M)
	for i := range msg {
		var valid bool
		dout[i], valid = ValidateStripHash(msg[i])
		if !valid {
			t.Errorf("Hash not valid for packet %d.\n", i)
		}
	}

	packets := make([]*Packet, K+M)
	for i := range dout {
		var p Packet
		err := proto.Unmarshal(dout[i], &p)
		if err != nil {
			t.Errorf("Could not Unmarshal packet %d\n", i)

		}
		packets[i] = &p
	}

	msgData := make([][]byte, K+M)
	for i := range packets {
		msgData[i] = packets[i].Data
	}

	enc, _ := reedsolomon.New(K, M)

	ok, _ := enc.Verify(msgData)
	if !ok {
		t.Errorf("Could not verify encoded values\n")
	}

	for i := 0; i < M; i++ {
		msgData[rand.Intn(M)] = nil
	}

	err = enc.Reconstruct(msgData)
	if err != nil {
		t.Errorf("Failure to reconstruct data\n")
	}

	buf := new(bytes.Buffer)

	enc.Join(buf, msgData, int(packets[0].Size))

	if !bytes.Equal(buf.Bytes(), din[:K*PktSize]) {
		t.Errorf("Sent data does not match received data.\n")
	}

}

func TestPacketWirePacket(t *testing.T) {
	p := &Packet{
		uint32(rand.Int()),
		uint32(rand.Int()),
		uint32(rand.Int()),
		uint32(rand.Int()),
		uint32(rand.Int()),
		make([]byte, rand.Intn(PktSize)),
	}
	rand.Read(p.Data)

	data, err := PacketToWire(p)
	if err != nil {
		t.Errorf("Could not convert Packet to wire data: %s\n", err)
	}

	result, err := WireToPacket(data)
	if err != nil {
		t.Fatalf("Could not convert wire data to Packet: %s\n", err)
	}

	if !reflect.DeepEqual(p, result) {
		t.Errorf("Packets are not the same!\n")
	}

	data[rand.Intn(len(data))] = data[rand.Intn(len(data))] ^ 0xff

	result, err = WireToPacket(data)
	if err == nil {
		t.Fatalf("Packet converted when should not have\n")
	}

}
