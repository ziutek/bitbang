package spi

import "io"

// Mode describes relation between bit read/write and clock value/edge and what bit
// from byte need to be transmitted first..
type Mode int

const (
	MSBF  Mode = 0 // Most significant bit first.
	LSBF  Mode = 1 // Least significant bit first.
	CPOL0 Mode = 0 // Clock idle state is 0.
	CPOL1 Mode = 2 // Clock idle state is 1.
	CPHA0 Mode = 0 // Sample on first edge.
	CPHA1 Mode = 4 // Sample on second edge.
)

// Config contains configuration of SPI mster. There can be many types of
// slave devices connected to the SPI bus. Any device may require specific
// master configuration to communicate properly.
type Config struct {
	Mode     Mode
	FrameLen int // Number of bits in frame.
	Delay    int // Delay between frames (t = Delay * 4 * clock_period).
}

// SPI implements Serial Peripheral Interface protocol on master side
// (see: http://en.wikipedia.org/wiki/Serial_Peripheral_Interface_Bus).
type SPI struct {
	rw         io.ReadWriter
	cfg        Config
	n          int
	mosi, miso byte
	obuf       [17]byte
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
	spi.cfg = Config{FrameLen: 8}
	spi.n = 0
	spi.mosi = mosi
	spi.miso = miso
	for i := 1; i < len(spi.obuf); i += 2 {
		spi.obuf[i] = sclk
	}
}

// Configure configures SPI master. It can be used before every conversation
// to specific slave device.
func (spi *SPI) Configure(cfg Config) {
	spi.cfg = cfg
}

// NewFrame resets internal bit counter. Use it to ensure that subsequent
// Write starts new frame. Additionaly it ensures that subsequent Write call
// doesn't start with delay clock sequence. Use it to avoid delay when
// it isn't need.
func (spi *SPI) NewFrame() {
	spi.n = 0
}

// Write
// Write requires that SCLK line is in proper idle state.
func (spi *SPI) Write(data []byte) (n int, err error) {
	if n = 16 {
		
	}
	if n == 0 {
		// New word begins.
		
	}

	var m int
	if spi.mode&CPHA1 != 0 {

		m, err = spi.rw.Write(spi.base)
		n += m
		if err != nil {
			return
		}
	}
	var (
		mask  uint
		shift func(uint) uint
	)
	if spi.mode&LSBF == 0 {
		mask = 0x01
		shift = func(m uint) uint { return m << 1 }
	} else {
		mask = 0x80
		shift = func(m uint) uint { return m >> 1 }
	}
	for b := range data {
		for i := 0; i < 16; i += 2 {
			if uint(b)&mask == 0 {
				spi.obuf[i] &^= spi.mosi
				spi.obuf[i+1] &^= spi.mosi
			} else {
				spi.obuf[i] |= spi.mosi
				spi.obuf[i+1] |= spi.mosi
			}
			mask = shift(mask)
		}
		m, err = spi.rw.Write(spi.obuf[:])
		n += m
		if err != nil {
			return
		}
	}
	if spi.mode&CPHA1 == 0 {
		m, err = spi.rw.Write(spi.base)
		n += m
		if err != nil {
			return
		}
	}
	return
}
