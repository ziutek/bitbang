// Package SPI implements SPI protocol (http://en.wikipedia.org/wiki/Serial_Peripheral_Interface_Bus)
// using bit banging (http://en.wikipedia.org/wiki/Bit_banging).
package spi

import (
	"github.com/ziutek/bitbang"
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
	FrameLen int // Number of bytes in frame.
	Delay    int // Delay between frames (delayTime = Delay*clockPeriod).
}

// SPI implements Serial Peripheral Interface protocol on master side.
type SPI struct {
	drv  bitbang.SyncDriver
	rbif int
	wbif int
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
func New(drv bitbang.SyncDriver, sclk, mosi, miso byte) *SPI {
	spi := new(SPI)
	spi.Init(drv, sclk, mosi, miso)
	return spi
}

// Init works like New but initializes existing SPI variable.
func (spi *SPI) Init(drv bitbang.SyncDriver, sclk, mosi, miso byte) {
	if sclk&mosi != 0 {
		panic("SPI.Init: scln&mosi != 0")
	}
	spi.drv = drv
	spi.rbif = 0
	spi.wbif = 0
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

// Begin should be called before starting conversation with slave device. It
// calls drivers PurgeReadBuffer method and in case of CPHA1 it sets SCLK line
// to its idle state.  If no erros and ss!=nil Begin calls ss(true) just before
// return. ss can be used to select slave device.
func (spi *SPI) Begin(ss func(bool) error) error {
	if err := spi.drv.PurgeReadBuffer(); err != nil {
		return err
	}
	spi.wbif = 0
	if spi.cpha1 {
		if _, err := spi.drv.Write([]byte{spi.cidle}); err != nil {
			return err
		}
		spi.rbif = -1 // Subsequent Read should skip first byte from driver.
	} else {
		spi.rbif = 0
	}
	if ss != nil {
		return ss(true)
	}
	return nil
}

// End should be called after end of conversation with slave device. In case of
// CPHA1 it sets SCLK line to its idle state. After that it calls driver's Flush
// method to ensure that all bits are really sent. If no errors and ss!=nil End
// calls ss(false) just before return. ss can be used to deselect slave device.
func (spi *SPI) End(ss func(bool) error) error {
	if !spi.cpha1 {
		if _, err := spi.drv.Write([]byte{spi.cidle}); err != nil {
			return err
		}
	}
	if err := spi.drv.Flush(); err != nil {
		return err
	}
	if ss != nil {
		return ss(true)
	}
	return nil
}

//  NoDelay provides way to avoid produce delay bits before write new frame if
// they are not needed this time.
func (spi *SPI) NoDelay() {
	spi.rbif = 0
	spi.wbif = 0
}

// Write writes data to SPI bus. It divides data into frames and generates
// delay bits if need. It uses driver's Write method with no more than 16 bytes
// at once. Driver should implement buffering if such small chunks degrades
// performance.
func (spi *SPI) Write(data []byte) (int, error) {
	var buf [16]byte
	mask := uint(0x80)
	if spi.lsbf {
		mask = uint(0x01)
	}
	for k, b := range data {
		if spi.delay != 0 {
			if spi.wbif == spi.flen {
				for i := 0; i < spi.delay; i++ {
					if _, err := spi.drv.Write([]byte{spi.cidle, spi.cidle}); err != nil {
						return k, err
					}
				}
				spi.wbif = 0
			}
			spi.wbif++
		}
		u := uint(b)
		for i := 0; i < 16; i += 2 {
			out := spi.cfirst
			if mask&u != 0 {
				out = spi.cfirst | spi.mosi
			}
			buf[i] = out
			buf[i+1] = out ^ spi.sclk
			if spi.lsbf {
				u >>= 1
			} else {
				u <<= 1
			}
		}
		if _, err := spi.drv.Write(buf[:]); err != nil {
			return k, err
		}
	}
	return len(data), nil
}

// WriteN writes n zero bytes to SPI bus.
func (spi *SPI) WriteN(n int) (int, error) {
	var buf [16]byte
	for k := 0; k < n; k++ {
		if spi.delay != 0 {
			if spi.wbif == spi.flen {
				for i := 0; i < spi.delay; i++ {
					if _, err := spi.drv.Write([]byte{spi.cidle, spi.cidle}); err != nil {
						return k, err
					}
				}
				spi.wbif = 0
			}
			spi.wbif++
		}
		for i := 0; i < 16; i += 2 {
			out := spi.cfirst
			buf[i] = out
			buf[i+1] = out ^ spi.sclk
		}
		if _, err := spi.drv.Write(buf[:]); err != nil {
			return k, err
		}
	}
	return n, nil
}

func (spi *SPI) Read(data []byte) (int, error) {
	var buf [16]byte
	if spi.rbif != -1 {
		
	}
	for k := range data {

	}
}
