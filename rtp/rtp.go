package rtp

import (
	"fmt"
	"log"
	"net"
)

const (
	RTP_VERSION = 2
)

const (
	hasRtpPadding = 1 << 2
	hasRtpExt     = 1 << 3
)

// Packet as per https://tools.ietf.org/html/rfc1889#section-5.1
//
//  0                   1                   2                   3
//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |V=2|P|X|  CC   |M|     PT      |       sequence number         |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                           timestamp                           |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |           synchronization source (SSRC) identifier            |
// +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
// |            contributing source (CSRC) identifiers             |
// |                             ....                              |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

type JPEGPacket struct {
	TypeSpecific   bool
	FragmentOffset uint
	Type           byte
	Width          uint
	Height         uint
	Payload        []byte
}

type RtpPacket struct {
	Version        byte
	Padding        bool
	Ext            bool
	Marker         bool
	PayloadType    byte
	SequenceNumber uint
	Timestamp      uint
	SyncSource     uint
	JPEGPacket     JPEGPacket
}

type Session struct {
	Rtp  net.PacketConn
	Rtcp net.PacketConn

	RtpChan  <-chan RtpPacket
	RtcpChan <-chan []byte

	rtpChan  chan<- RtpPacket
	rtcpChan chan<- []byte

	Control chan bool
}

func New(rtp, rtcp net.PacketConn) *Session {
	rtpChan := make(chan RtpPacket, 1000)
	ctrlChan := make(chan bool)
	//rtcpChan := make(chan []byte, 1000)
	s := &Session{
		Rtp:     rtp,
		Rtcp:    rtcp,
		RtpChan: rtpChan,
		Control: ctrlChan,
		//RtcpChan: rtcpChan,
		rtpChan: rtpChan,
	}

	go s.HandleRtpConn(rtp)
	//go s.HandleRtcpConn(rtcp)
	return s
}

func toUint(arr []byte) (ret uint) {
	for i, b := range arr {
		ret |= uint(b) << (8 * uint(len(arr)-i-1))
	}
	return ret
}

func (s *Session) HandleRtpConn(conn net.PacketConn) {
	buf := make([]byte, 9000)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			s.Control <- true
			log.Println("RTP Connection closed")
			return
		}

		cpy := make([]byte, n)
		copy(cpy, buf)
		s.handleRtp(cpy)
	}
}

func (s *Session) HandleRtcpConn(conn net.PacketConn) {
	buf := make([]byte, 10000)

	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}

		cpy := make([]byte, n)
		copy(cpy, buf)
		s.handleRtcp(cpy)
	}
}

func (s *Session) handleRtp(buf []byte) {
	packet := RtpPacket{
		Version:        buf[0] >> 6 & 0x03,
		Padding:        buf[0]&hasRtpPadding != 0,
		Marker:         buf[1]>>7&1 != 0,
		PayloadType:    buf[1] >> 1,
		SequenceNumber: toUint(buf[2:4]),
		Timestamp:      toUint(buf[4:8]),
		SyncSource:     toUint(buf[8:12]),
	}

	if packet.Version != RTP_VERSION {
		log.Println("Wrong version")
		return

	}

	if len(buf) < 12 {
		log.Println("short packet")
		return
	}

	jpegPacket := JPEGPacket{
		TypeSpecific:   buf[12]&1 != 0,
		FragmentOffset: toUint(buf[13:16]),
		Type:           buf[17],
		Width:          uint(buf[18]) * 8,
		Height:         uint(buf[19]) * 8,
		Payload:        buf[20:],
	}

	packet.JPEGPacket = jpegPacket
	s.rtpChan <- packet

}

func (s *Session) handleRtcp(buf []byte) {
	// TODO: implement rtcp
	fmt.Println("RTCP Message")
}
