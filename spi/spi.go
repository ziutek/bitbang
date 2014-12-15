// Package SPI implements SPI protocol (http://en.wikipedia.org/wiki/Serial_Peripheral_Interface_Bus)
// using bit banging (http://en.wikipedia.org/wiki/Bit_banging).
package spi

import "io"

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

// Config contains configuration of SPI mster. There can be many types of slave
// devices connected to the SPI bus. Any device may require specific master
// configuration to communicate properly.
type Config struct {
	Mode     Mode
	FrameLen int // Number of bits in frame.
	Delay    int // Delay between frames (t = Delay * 4 * clock_period).
}

// SPI implements Serial Peripheral Interface protocol on master side.
type SPI struct {
	rw   io.ReadWriter
	n    int
	sclk byte
	mosi byte
	miso byte

	// From Config.
	cidle  byte
	cfirst byte
	lsbf   bool
	flen   int
}

// New returns new SPI that uses rw to read/write data using SPI protocol.
// Every byte that is read/written to rw is treated as word of bits
// that are samples of SCLK, MOSI and MISO lines. For example
// byte&mosi != 0 means that MOSI line is high.
// New panics if sclk&mosi != 0. Default configuration is: MSBF, CPOL0,
// CPHA0, 8bit, no delay.
// TODO: Write about sampling rate.
func New(rw io.ReadWriter, sclk, mosi, miso byte) *SPI {
	spi := new(SPI)
	spi.Init(rw, sclk, mosi, miso)
	return spi
}

// Init works like New but initializes existing SPI variable.
func (spi *SPI) Init(rw io.ReadWriter, sclk, mosi, miso byte) {
	if sclk&mosi != 0 {
		panic("scln&mosi != 0")
	}
	spi.rw = rw
	spi.n = 0
	spi.sclk = sclk
	spi.mosi = mosi
	spi.miso = miso
	spi.flen = 8
}

// Configure configures SPI master. It can be used before start conversation to
// slave device.
func (spi *SPI) Configure(cfg Config) {
	if cfg.Mode&CPHA1 == 0 {
		spi.cidle = 0
		if cfg.Mode&CPOL1 == 0 {
			spi.cfirst = 0
		} else {
			spi.cfirst = spi.sclk
		}
	} else {
		spi.cidle = spi.sclk
		if cfg.Mode&CPOL1 == 0 {
			spi.cfirst = spi.sclk
		} else {
			spi.cfirst = 0
		}
	}
	spi.lsbf = (cfg.Mode&LSBF != 0)
}

// Idle sets SCLK line in idle state. It should be called before starting
// conversation (for CPHA1) or after ended conversation (for CPHA0).
func (spi *SPI) Idle() error {
	_, err := spi.rw.Write([]byte{spi.cidle})
	return err
}

// NewFrame resets internal bit counter. Use it to ensure that subsequent Write
// starts new frame (not need if last frame has been completly written).
// Additionaly it ensures that subsequent Write call doesn't start with delay
// clock sequence. Use it to avoid produce delay bits if they aren't needed.
func (spi *SPI) NewFrame() {
	spi.n = 0
}

// Write
// Write requires that SCLK line is in proper idle state.
func (spi *SPI) Write(data []byte) (int, error) {
	var obuf [16]byte
	if spi.lsbf {
		for n, b := range data {
			mask := uint(0x01)
			for i := 0; i < 16; i += 2 {
				out := spi.cfirst
				if mask&uint(b) != 0 {
					out = spi.cfirst | spi.mosi
				}
				obuf[i] = out
				obuf[i+1] = out ^ spi.sclk
				mask <<= 1
			}
			if _, err := spi.rw.Write(obuf[:]); err != nil {
				return n, err
			}
		}
	} else {
		for n, b := range data {
			mask := uint(0x80)
			for i := 0; i < 16; i += 2 {
				out := spi.cfirst
				if mask&uint(b) != 0 {
					out = spi.cfirst | spi.mosi
				}
				obuf[i] = out
				obuf[i+1] = out ^ spi.sclk
				mask >>= 1
			}
			if _, err := spi.rw.Write(obuf[:]); err != nil {
				return n, err
			}
		}
	}
	return len(data), nil
}
