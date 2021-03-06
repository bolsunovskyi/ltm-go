package ltm

import (
	"log"

	"github.com/tarm/serial"
)

const (
	serialStateIDLE = iota
	serialStateHeaderStart1
	serialStateHeaderStart2
	serialStateHeaderMsgType
	//serialStateHeaderData

	gFrameLength = 18
	aFrameLength = 10
	sFrameLength = 11
	oFrameLength = 18
	nFrameLength = 10
	xFrameLength = 10

	gFrame = "G"
	aFrame = "A"
	sFrame = "S"
	oFrame = "O"
	nFrame = "N"
	xFrame = "X"

	SatFix2D = 2
	SatFix3D = 3
)

type LTM struct {
	port   *serial.Port
	cState int

	//sFrame
	uavBat               uint16
	uavAmp               uint16
	uavFixType           uint8 // GPS lock 0-1=no fix, 2=2D, 3=3D
	uavSatellitesVisible uint8

	//gFrame
	uavLat int32
	uavLon int32
}

func (l LTM) IsSat2DFix() bool {
	return l.uavFixType == SatFix2D
}

func (l LTM) IsSat3DFix() bool {
	return l.uavFixType == SatFix3D
}

func (l LTM) GetSatFix() uint8 {
	return l.uavFixType
}

func (l LTM) GetSatellitesVisible() uint8 {
	return l.uavSatellitesVisible
}

func (l LTM) GetBat() uint16 {
	return l.uavBat
}

func (l LTM) GetAmp() uint16 {
	return l.uavAmp
}

func (l LTM) GetGPS() (lat, lon int32) {
	return l.uavLat, l.uavLon
}

func Make(port string, baud int) (*LTM, error) {
	c := &serial.Config{Name: port, Baud: baud}
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}

	return &LTM{
		port:   s,
		cState: serialStateIDLE,
	}, nil
}

func toUInt32(buf []byte, startIndex int) uint32 {
	v := uint32(buf[startIndex])
	v |= uint32(buf[startIndex+1]) << 8
	v |= uint32(buf[startIndex+2]) << 16
	v |= uint32(buf[startIndex+3]) << 24

	return v
}

func (l *LTM) parseFrame(cmd string, serialBuffer []byte) {
	if cmd == sFrame {
		l.uavBat = uint16(serialBuffer[0])
		l.uavBat |= uint16(serialBuffer[1]) << 8
	} else if cmd == gFrame {
		l.uavLat = int32(toUInt32(serialBuffer, 0))
		l.uavLon = int32(toUInt32(serialBuffer, 4))

		satsFix := uint8(serialBuffer[13])
		l.uavSatellitesVisible = (satsFix >> 2) & 0xFF
		l.uavFixType = satsFix & 0x3
	}
}

func (l *LTM) Read() {
	buf := make([]byte, 1)
	var (
		frameLength   uint8
		cmd           string
		receiverIndex uint8
		rcvChecksum   uint8
	)

	serialBuffer := make([]byte, gFrameLength-4)

	for {
		n, err := l.port.Read(buf)
		if err != nil {
			log.Println(err)
			return
		}
		c := string(buf[0])

		if n > 0 {
			if l.cState == serialStateIDLE {
				if c == "$" {
					l.cState = serialStateHeaderStart1
				}
			} else if l.cState == serialStateHeaderStart1 {
				if c == "T" {
					l.cState = serialStateHeaderStart2
				}
			} else if l.cState == serialStateHeaderStart2 {
				switch c {
				case gFrame:
					frameLength = gFrameLength
					l.cState = serialStateHeaderMsgType
					//log.Println("g frame")
				case aFrame:
					frameLength = aFrameLength
					l.cState = serialStateHeaderMsgType
					//log.Println("a frame")
				case sFrame:
					frameLength = sFrameLength
					l.cState = serialStateHeaderMsgType
					//log.Println("s frame")
				case oFrame:
					frameLength = oFrameLength
					l.cState = serialStateHeaderMsgType
					//log.Println("o frame")
				case nFrame:
					frameLength = nFrameLength
					l.cState = serialStateHeaderMsgType
					//log.Println("n frame")
				case xFrame:
					frameLength = xFrameLength
					l.cState = serialStateHeaderMsgType
					//log.Println("x frame")
				default:
					l.cState = serialStateIDLE
				}

				cmd = c
				receiverIndex = 0
			} else if l.cState == serialStateHeaderMsgType {
				if receiverIndex == 0 {
					rcvChecksum = buf[0]
				} else {
					rcvChecksum ^= buf[0]
				}

				if receiverIndex == frameLength-4 {
					l.cState = serialStateIDLE

					if rcvChecksum == 0 {
						//PARSE BUFFER
						l.parseFrame(cmd, serialBuffer)
						//log.Println("frame received, ready to parse")
					} else {
						//checksum error
						log.Println("frame checksum error")
					}
				} else {
					serialBuffer[receiverIndex] = buf[0]
					receiverIndex += 1
				}
			}
		}
	}
}
