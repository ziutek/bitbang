// Package SPI implements SPI protocol (http://en.wikipedia.org/wiki/Serial_Peripheral_Interface_Bus)
// using bit banging (http://en.wikipedia.org/wiki/Bit_banging).
package spi

import (
	"io"
)

// Mode describes relation between bit read/write and clock value/edge and what
// bit from a byte need to be transmitted first.
type Mode int

const (
	MSBF  Mode = 0 // Most significant bit first.
	LSBF  Mode = 1 // Least significant bit first.
	CPOL0 Mode = 0 // Clock idle state is 0.
	CPOL1 Mode = 2 // Clock idle state is 1.
	CPHA0 Mode = 0 // Sample on first edge.
	CPHA1 Mode = 4 // Sample on second edge.
)

func (m Mode) String() string {
	switch m {
	case MSBF | CPOL0 | CPHA0:
		return "M00"
	case MSBF | CPOL0 | CPHA1:
		return "M01"
	case MSBF | CPOL1 | CPHA0:
		return "M10"
	case MSBF | CPOL1 | CPHA1:
		return "M11"
	case LSBF | CPOL0 | CPHA0:
		return "L00"
	case LSBF | CPOL0 | CPHA1:
		return "L01"
	case LSBF | CPOL1 | CPHA0:
		return "L10"
	case LSBF | CPOL1 | CPHA1:
		return "L11"
	}
	return "unknown"
}

// Config contains configuration of SPI mster. There can be many types of slave
// devices connected to the SPI bus simultaneously. Any device may require
// specific master configuration to communicate properly.
type Config struct {
	Mode     Mode
	FrameLen int // Number of bytes in frame (only used if Delay!=0).
	Delay    int // Delay between frames (delayTime = Delay*clockPeriod).
}

// SPI implements Serial Peripheral Interface protocol on master side.
type SPI struct {
	r    io.Reader
	w    io.Writer
	bif  int
	sclk byte
	mosi byte
	miso byte

	// From Config.
	cidle  byte
	cfirst byte
	cpha1  bool
	lsbf   bool
	flen   int
	delay  int
}

// New returns new SPI that uses r/w to read/write data using SPI protocol.
// Every byte that is read/written to r/w is treated as word of bits
// that are samples of SCLK, MOSI and MISO lines. For example
// byte&mosi != 0 means that MOSI line is high.
// New panics if sclk&mosi != 0. Default configuration is: MSBF, CPOL0,
// CPHA0, 8bit, no delay.
// TODO: Write about sampling rate.
func New(r io.Reader, w io.Writer, sclk, mosi, miso byte) *SPI {
	spi := new(SPI)
	spi.Init(r, w, sclk, mosi, miso)
	return spi
}

// Init works like New but initializes existing SPI variable.
func (spi *SPI) Init(r io.Reader, w io.Writer, sclk, mosi, miso byte) {
	if sclk&mosi != 0 {
		panic("SPI.Init: scln&mosi != 0")
	}
	spi.r = r
	spi.w = w
	spi.bif = 0
	spi.sclk = sclk
	spi.mosi = mosi
	spi.miso = miso
}

// Configure configures SPI master. It can be used before start conversation to
// slave device.
func (spi *SPI) Configure(cfg Config) {
	spi.cpha1 = (cfg.Mode&CPHA1 != 0)
	if cfg.Mode&CPOL1 == 0 {
		spi.cidle = 0
		if spi.cpha1 {
			spi.cfirst = spi.sclk
		} else {
			spi.cfirst = 0
		}
	} else {
		spi.cidle = spi.sclk
		if spi.cpha1 {
			spi.cfirst = 0
		} else {
			spi.cfirst = spi.sclk
		}
	}
	spi.lsbf = (cfg.Mode&LSBF != 0)
	if cfg.Delay < 0 {
		panic("SPI.Configure: delay < 0")
	}
	if cfg.Delay != 0 && cfg.FrameLen <= 0 {
		panic("SPI.Configure: delay != 0 && flen <= 0")
	}
	spi.flen = cfg.FrameLen
	spi.delay = cfg.Delay
}

// Begin should be called before starting conversation with slave device.
// If ss!=nil Begin calls ss(true) just before return, to select slave device.
func (spi *SPI) Begin(ss func(bool) error) error {
	spi.bif = 0
	if spi.cpha1 {
		if _, err := spi.w.Write([]byte{spi.cidle}); err != nil {
			return err
		}
	}
	if ss != nil {
		return ss(true)
	}
	return nil
}

// End should be called after end of conversation with slave device.
// If ss!=nil End calls ss(false) just before return, to deselect slave device.
func (spi *SPI) End(ss func(bool) error) error {
	spi.bif = 0
	if !spi.cpha1 {
		if _, err := spi.w.Write([]byte{spi.cidle}); err != nil {
			return err
		}
	}
	if ss != nil {
		return ss(true)
	}
	return nil
}

// NewFrame resets internal byte counter. Use it to ensure that subsequent Write
// starts new frame (not need if last frame has been completly written).
// Additionaly it ensures that subsequent Write doesn't start with delay
// sequence. Use it to avoid produce delay bits before new frame if they are not
// needed this time.
func (spi *SPI) NewFrame() {
	spi.bif = 0
}

// Write writes data to SPI bus. It divides data into frames and generates
// delay bits if need.
func (spi *SPI) Write(data []byte) (int, error) {
	var obuf [16]byte
	mask := uint(0x80)
	if spi.lsbf {
		mask = uint(0x01)
	}
	for n, b := range data {
		if spi.delay != 0 {
			if spi.bif == spi.flen {
				for i := 0; i < spi.delay; i++ {
					if _, err := spi.w.Write([]byte{spi.cidle, spi.cidle}); err != nil {
						return n, err
					}
				}
				spi.bif = 0
			}
			spi.bif++
		}
		u := uint(b)
		for i := 0; i < 16; i += 2 {
			out := spi.cfirst
			if mask&u != 0 {
				out = spi.cfirst | spi.mosi
			}
			obuf[i] = out
			obuf[i+1] = out ^ spi.sclk
			if spi.lsbf {
				u >>= 1
			} else {
				u <<= 1
			}
		}
		if _, err := spi.w.Write(obuf[:]); err != nil {
			return n, err
		}
	}
	return len(data), nil
}
