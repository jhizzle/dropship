package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"hash/crc32"
	"log"
	"math/rand"
)

var _ = crc32.ChecksumIEEE

func main() {

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

	hash := crc32.ChecksumIEEE(data)
	data = append(data, []byte(fmt.Sprintf("%08x", hash))...)

	fmt.Printf("Marshalled data is %d bytes\n", len(data))

	r := &Packet{}

	if err := proto.Unmarshal(data[:len(data)-8], r); err != nil {
		log.Fatalln("Failed to parse Packet:", err)
	}
	fmt.Printf("Readback: %v\n", r)
	fmt.Printf("Hash attached: %s\n", data[len(data)-8:])
	fmt.Printf("Hash Calculated: %08x\n", crc32.ChecksumIEEE(data[:len(data)-8]))

}
