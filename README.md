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

## Receiver

triage:
  receive packet
  hash ok? NO? Set flag and send to message handler
  does this message exist? NO? create message
  send packet to message handler

message handler:
  place data into message
  if we have enough data, decode it and send it to data handler
  keep it around for a little bit to handle any extra packets that come in for this message



```
A Shipper sends data to a Hub over the network.
func NewShipper(destAddr string) (*Shipper, error)

A Hub listens on a port and sends packets to the appropriate receiver.
func NewHub(listenAddr string) (*Hub, error)

Register a receiver on a route.
func (*Hub) RegisterReceiver(io.Writer) (chan err)

Request a new Receiver on a route.
func (*Hub) NewReceiver(route int) (*Receiver)


A Hub sends all packets destined for a route to that Route over a channel. The
channel acts as a buffer, so we must give it a certain capacity. When the Route
gets a packet, it sees which Id it has and checks to see if a Message with that
ID exists. If it does, it sends the packet to the Message. If not, it creates a
new Message with that ID and sends the packet to the Message. When the Message
is created, a timer is created that will timeout the message after a certain
amount of time.

A Route gets packets from a channel. The challenge is that packets can arrive
out of order. This isn't a problem if the packet is from the same message, but
if packets from other messages arrive, they have to be stored temporarily or
reassembled in their own messages. This is a problem because if multiple
messages are showing up, we need to time out when we think a message has
dropped too many packets. If we dropped packets, we need to make sure that we
send an error explaining this. It maybe still possible to continue even with
dropped packets. If multiple messages are being assembled, we also need to make
sure that the data is being output in the correct order.

```

New Idea? Instead of registering a receiver, just return a Route or Mailbox or
Receiver or something that people can Read from. That's probably more
realistic as to what people want anyways. Just have the Route buffer up the
data.

Receiver: ID, Read, Write.

