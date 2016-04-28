DropShip
========

Tool to send files with one-way communication.

* Send a directory of files recursively to a host.
* Receive files into a directory.

## Basic Theory

Data is sent as a "message" where a message is made up of a sequence of
packets with reed solomon encoding. Each packet contains information about the
message including: what message the packet belongs to, the sequence of the
packet in the message, the number of data and parity packets in the message,
and the size of the message.  And of course, each packet contains a part of
the message.

The reed solomon encoding allows a defined number of packets to be lost or
corrupted with the ability to still recover the message.

## Sending Data

the parameters K, M, and PktSize are constants

### Sender

takes a reader
keeps reading until all the data is gone
sends messages as long as it has data
sends the last message when EOF or error

a connection is made taking an io.Reader and a destination

func NewSender(dest string, data io.Reader) (*Sender, error)

func(s *Sender) Send(data []byte) (sent int)
