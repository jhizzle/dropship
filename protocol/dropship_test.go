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
	//"reflect"
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
		return
	}

	laddr, err := net.ResolveUDPAddr("udp", ":1712")
	if err != nil {
		t.Errorf("ResolveUDPAddr failed to resolve address: %s\n", err)
		return
	}
	r, err := net.ListenUDP("udp", laddr)
	if err != nil {
		t.Errorf("Failed to listen on udp: %s\n", err)
		return
	}

	msg := make([]byte, 2^20)
	rand.Read(msg)
	n, err := s.Write(msg)
	if n != len(msg) || err != nil {
		t.Errorf("Write:: Expected: (%d, nil), Actual: (%d, %v)\n", len(msg), n, err)
		return
	}

	buf := make([]byte, 5000)
	shards := make([][]byte, K+M)
	for i := 0; i < K+M; i++ {
		r.SetDeadline(time.Now().Add(time.Second))
		n, err = r.Read(buf)
		if err != nil {
			t.Errorf("Read fail: %s\n", err)
			break
		}
		shards[i] = append(shards[i], buf[:n]...)
	}

	result, err := DecodeMessage(shards)
	if err != nil {
		t.Errorf("Failed to DecodeMessage: %s\n", err)
		return
	}

	if !bytes.Equal(msg, result) {
		t.Errorf("Decoded message does not equal sent message\n")
		return
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

	for i, p := range packets {
		if p.Id != 1 {
			t.Errorf("Packet %d doesn't have correct ID. Expected: %d, Actual: %d\n", i, 1, p.Id)
		}
		if p.Seq != uint32(i) {
			t.Errorf("Packet %d doesn't have correct sequence. Expected: %d, Actual: %d\n", i, i, p.Seq)
		}
		if p.Size != uint32(n) {
			t.Errorf("Packet %d doesn't have correct size. Expected: %d, Actual: %d\n", i, n, p.Size)
		}

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

func TestDecodeMessage(t *testing.T) {

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

	result, err := DecodeMessage(msg)
	if err != nil {
		t.Errorf("Failed to decode message: %s\n", err)
	}

	if !bytes.Equal(din[:n], result) {
		t.Errorf("Decoded message is not equal to original\n")
	}

	// random drop
	rand.Read(din)
	msg, n = s.MakeMessage(din)
	if n != K*PktSize {
		t.Errorf("Didn't use maximum data. Expected: %d, Actual: %d\n", K*PktSize, n)
	}

	for i := 0; i < M; i++ {
		msg[rand.Intn(M)] = nil
	}

	result, err = DecodeMessage(msg)
	if err != nil {
		t.Errorf("Failed to decode message: %s\n", err)
	}

	if !bytes.Equal(din[:n], result) {
		t.Errorf("Decoded message is not equal to original\n")
	}

	// out of order and corrupted
	rand.Read(din)
	msg, n = s.MakeMessage(din)
	if n != K*PktSize {
		t.Errorf("Didn't use maximum data. Expected: %d, Actual: %d\n", K*PktSize, n)
	}

	for i := 0; i < M; i++ {
		shard := rand.Intn(K + M)
		b := rand.Intn(len(msg[shard]))
		msg[shard][b] = msg[shard][b] ^ 0xff
	}

	for i := 0; i < K+M; i++ {
		shard := rand.Intn(K + M)
		msg[i], msg[shard] = msg[shard], msg[i]
	}

	result, err = DecodeMessage(msg)
	if err != nil {
		t.Errorf("Failed to decode message: %s\n", err)
	}

	if !bytes.Equal(din[:n], result) {
		t.Errorf("Decoded message is not equal to original\n")
	}

}

func TestToMessageAndBack(t *testing.T) {
	testVectors := []struct {
		id        int
		k         int
		m         int
		shardSize int
		msgSize   int
		encErr    error
		dropped   int
		decErr    error
	}{
		{0, 0, 0, 0, 0, ArgumentError, 0, nil},
		{1, 5, 5, 10, 49, nil, 0, nil},
		{2, 5, 5, 10, 50, nil, 0, nil},
		{3, 5, 5, 10, 51, nil, 0, nil},
		{4, 1, 1, 1, 1, nil, 0, nil},
		{5, 200, 55, 1000, 50000, nil, 0, nil},
		{6, 200, 55, 1000, 500000, nil, 0, nil},
		{7, 20, 5, 10, 50000, nil, 4, nil},
		{8, 20, 5, 10, 50000, nil, 5, nil},
		{9, 20, 5, 10, 50000, nil, 6, reedsolomon.ErrTooFewShards},
	}

	for i, test := range testVectors {
		buf := make([]byte, test.msgSize)
		rand.Read(buf)

		m, err := DataToMessage(buf, test.id, test.k, test.m, test.shardSize)
		if err != test.encErr {
			t.Errorf("Test: %3d, DataToMessage expected error: %v, actual: %v\n", i, test.encErr, err)
			continue
		}

		if m == nil {
			continue
		}

		testSize := Min(test.msgSize, test.k*test.shardSize)
		if m.Size != testSize {
			t.Errorf("Test: %3d, Message size is wrong. Expected: %d, actual: %d\n", i, testSize, m.Size)
		}

		// drop packets
		for i := 0; i < Min(test.dropped, test.k+test.m); i++ {

			for {
				shard := rand.Intn(len(m.Shards))
				if m.Shards[shard] == nil {
					continue
				}
				m.Shards[shard] = nil
				break
			}
		}

		result, err := DataFromMessage(m)
		if err != test.decErr {
			t.Errorf("Test: %3d, DataFromMessage Expected error: %v, actual: %v\n", i, test.encErr, err)
			continue
		}

		if err != nil {
			continue
		}

		if !bytes.Equal(buf[:m.Size], result) {
			t.Errorf("Test: %3d, DataFromMessage result data not equal to input data. Expected length: %d, actual: %d\n", i, m.Size, len(result))
		}

	}

}

func TestMessageToPacketAndBack(t *testing.T) {
	testVectors := []struct {
		id        int
		k         int
		m         int
		shardSize int
		msgSize   int
		encErr    error
		dropped   int
		shuffle   int
		decErr    error
	}{
		{0, 0, 0, 0, 0, ArgumentError, 0, 0, nil},
		{1, 5, 5, 10, 49, nil, 0, 0, nil},
		{2, 5, 5, 10, 50, nil, 0, 0, nil},
		{3, 5, 5, 10, 51, nil, 0, 0, nil},
		{4, 1, 1, 1, 1, nil, 0, 0, nil},
		{5, 200, 55, 1000, 50000, nil, 0, 0, nil},
		{6, 200, 55, 1000, 500000, nil, 0, 0, nil},
		{7, 20, 5, 10, 50000, nil, 4, 0, nil},
		{8, 20, 5, 10, 50000, nil, 5, 0, nil},
		{9, 20, 5, 10, 50000, nil, 6, 0, reedsolomon.ErrTooFewShards},
		{10, 20, 5, 10, 50000, nil, 4, 1, nil},
		{11, 20, 5, 10, 50000, nil, 5, 5, nil},
		{12, 20, 5, 10, 50000, nil, 6, 10, reedsolomon.ErrTooFewShards},
		{13, 20, 5, 10, 5000, nil, 5, 10, nil},
	}

	for i, test := range testVectors {
		buf := make([]byte, test.msgSize)
		rand.Read(buf)
		original := make([]byte, test.msgSize)
		copy(original, buf)

		m, _ := DataToMessage(buf, test.id, test.k, test.m, test.shardSize)
		packets, err := MessageToPackets(m)
		if err != test.encErr {
			t.Errorf("Test %3d: MessageToPackets: Expected error: %v, actual: %v\n", i, test.encErr, err)
			continue
		}

		if packets == nil {
			continue
		}

		// drop packets
		for i := 0; i < Min(test.dropped, test.k+test.m); i++ {

			for {
				shard := rand.Intn(len(m.Shards))
				if packets[shard] == nil {
					continue
				}
				packets[shard] = nil
				break
			}
		}

		// shuffle
		for i := 0; i < test.shuffle; i++ {
			x := rand.Intn(len(packets))
			y := rand.Intn(len(packets))

			packets[x], packets[y] = packets[y], packets[x]
		}

		resultMessage, err := MessageFromPackets(packets)
		if err != test.decErr {
			t.Errorf("Test %3d: MessageFromPackets expected error: %v, actual: %v\n", i, test.decErr, err)
			continue
		}

		if resultMessage == nil {
			continue
		}

		result, err := DataFromMessage(resultMessage)
		if err != nil {
			t.Errorf("Test %3d: failed to get DataFromMessage\n", i)
		}

		if !bytes.Equal(original[:m.Size], result) {
			t.Errorf("Test %3d: DataFromMessage result data not equal to input data. Expected length: %d, actual: %d\n", i, m.Size, len(result))
		}

	}
}

func TestPacketWirePacket(t *testing.T) {
	testVectors := []struct {
		id        int
		k         int
		m         int
		shardSize int
		msgSize   int
		shuffle   int
		dropped   int
		DTMErr    error
		DFMErr    error
		MTPErr    error
		MFPErr    error
	}{
		//id , k , m , shardSize , msgSize , shuffle , dropped , DTMErr , DFMErr , MTPErr , MFPErr
		{0, 0, 0, 0, 0, 0, 0, ArgumentError, nil, nil, nil},
		{1, 5, 5, 10, 49, 0, 0, nil, nil, nil, nil},
		{2, 5, 5, 10, 50, 0, 0, nil, nil, nil, nil},
		{3, 5, 5, 10, 51, 0, 0, nil, nil, nil, nil},
		{4, 1, 1, 1, 1, 0, 0, nil, nil, nil, nil},
		{5, 200, 55, 1000, 50000, 0, 0, nil, nil, nil, nil},
		{6, 200, 55, 1000, 500000, 0, 0, nil, nil, nil, nil},
		{7, 20, 5, 10, 50000, 0, 4, nil, nil, nil, nil},
		{8, 20, 5, 10, 50000, 0, 5, nil, nil, nil, nil},
		{9, 20, 5, 10, 50000, 0, 6, nil, nil, nil, reedsolomon.ErrTooFewShards},
		{10, 20, 5, 10, 50000, 1, 4, nil, nil, nil, nil},
		{11, 20, 5, 10, 50000, 5, 5, nil, nil, nil, nil},
		{12, 20, 5, 10, 50000, 10, 6, nil, nil, nil, reedsolomon.ErrTooFewShards},
		{13, 20, 5, 10, 5000, 10, 5, nil, nil, nil, nil},
	}

	for i, test := range testVectors {
		buf := make([]byte, test.msgSize)
		rand.Read(buf)
		original := make([]byte, test.msgSize)
		copy(original, buf)

		messageIn, err := DataToMessage(buf, test.id, test.k, test.m, test.shardSize)
		if err != test.DTMErr {
			t.Errorf("Test %3d: DataToMessage error: Expected: %v, actual: %v\n", i, test.DTMErr, err)
			continue
		}
		if messageIn == nil {
			continue
		}

		packets, err := MessageToPackets(messageIn)
		if err != test.MTPErr {
			t.Errorf("Test %3d: MessageToPackets error: Expected: %v, actual: %v\n", i, test.MTPErr, err)
			continue
		}
		if packets == nil {
			continue
		}

		wire := make([][]byte, test.k+test.m)
		for i, p := range packets {
			wire[i], err = PacketToWire(p)
			if err != nil {
				t.Errorf("Test %3d: Unexpected error from PacketToWire\n", i)
			}
		}

		// shuffle
		for i := 0; i < test.shuffle; i++ {
			x := rand.Intn(len(wire))
			y := rand.Intn(len(wire))

			wire[x], wire[y] = wire[y], wire[x]
		}

		// corrupt packets
		corrupted := make([]bool, len(wire))
		for i := 0; i < Min(test.dropped, test.k+test.m); i++ {
			packetToCorrupt := rand.Intn(len(wire))
			for ; corrupted[packetToCorrupt]; packetToCorrupt = rand.Intn(len(wire)) {
			}

			byteToCorrupt := rand.Intn(len(wire[packetToCorrupt]))
			wire[packetToCorrupt][byteToCorrupt] = wire[packetToCorrupt][byteToCorrupt] ^ 0xff
			corrupted[packetToCorrupt] = true
		}

		resultPackets := make([]*Packet, len(wire))
		for i, b := range wire {
			var corrupt bool
			resultPackets[i], err = PacketFromWire(b)
			if err != nil {
				corrupt = true
			} else {
				corrupt = false
			}

			if corrupted[i] != corrupt {
				t.Errorf("Test %3d: Expected packet corruption: %v, Actual packet corruption: %v\n", i, corrupted[i], corrupt)
			}
		}

		resultMessage, err := MessageFromPackets(resultPackets)
		if err != test.MFPErr {
			t.Errorf("Test %3d: MessageFromPackets error: Expected: %v, actual: %v\n", i, test.MFPErr, err)
			continue
		}
		if resultMessage == nil {
			continue
		}

		result, err := DataFromMessage(resultMessage)
		if err != test.DFMErr {
			t.Errorf("Test %3d: DataFromMessag error: Expected: %v, actual: %v\n", i, test.DFMErr, err)
			continue
		}
		if result == nil {
			continue
		}

		if !bytes.Equal(original[:resultMessage.Size], result) {
			t.Errorf("Test %3d: DataFromMessage result data not equal to input data. Expected length: %d, actual: %d\n", i, resultMessage.Size, len(result))
		}

	}
}
