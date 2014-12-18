// Package spi implements SPI protocol (http://en.wikipedia.org/wiki/Serial_Peripheral_Interface_Bus)
// using bit banging (http://en.wikipedia.org/wiki/Bit_banging).
package spi

import (
	"errors"
	"io"

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

// Master implements Serial Peripheral Interface protocol on master side.
type Master struct {
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
func NewMaster(drv bitbang.SyncDriver, sclk, mosi, miso byte) *Master {
	ma := new(Master)
	ma.Init(drv, sclk, mosi, miso)
	return ma
}

// Init works like New but initializes existing SPI variable.
func (ma *Master) Init(drv bitbang.SyncDriver, sclk, mosi, miso byte) {
	if sclk&mosi != 0 {
		panic("SPI.Init: scln&mosi != 0")
	}
	ma.drv = drv
	ma.rbif = 0
	ma.wbif = 0
	ma.sclk = sclk
	ma.mosi = mosi
	ma.miso = miso
}

// Configure configures SPI master. It can be used before start conversation to
// slave device.
func (ma *Master) Configure(cfg Config) {
	ma.cpha1 = (cfg.Mode&CPHA1 != 0)
	if cfg.Mode&CPOL1 == 0 {
		ma.cidle = 0
		if ma.cpha1 {
			ma.cfirst = ma.sclk
		} else {
			ma.cfirst = 0
		}
	} else {
		ma.cidle = ma.sclk
		if ma.cpha1 {
			ma.cfirst = 0
		} else {
			ma.cfirst = ma.sclk
		}
	}
	ma.lsbf = (cfg.Mode&LSBF != 0)
	if cfg.Delay < 0 {
		panic("SPI.Configure: delay < 0")
	}
	if cfg.Delay != 0 && cfg.FrameLen <= 0 {
		panic("SPI.Configure: delay != 0 && flen <= 0")
	}
	ma.flen = cfg.FrameLen
	ma.delay = cfg.Delay
}

// Begin should be called before starting conversation with slave device. It
// calls drivers PurgeReadBuffer method and in case of CPHA1 it sets SCLK line
// to its idle state. If no erros and ss!=nil Begin calls ss(true) just before
// return.
func (ma *Master) Begin(ss func(bool) error) error {
	if err := ma.drv.PurgeReadBuffer(); err != nil {
		return err
	}
	ma.wbif = 0
	if ma.cpha1 {
		if _, err := ma.drv.Write([]byte{ma.cidle}); err != nil {
			return err
		}
		ma.rbif = -1 // Subsequent Read should skip first byte from driver.
	} else {
		ma.rbif = 0
	}
	if ss != nil {
		return ss(true)
	}
	return nil
}

// End should be called after end of conversation with slave device. In case of
// CPHA1 it sets SCLK line to its idle state. After that it calls driver's
// Flush method to ensure that all bits are really sent. If no errors and
// ss!=nil End calls ss(false) just before return.
func (ma *Master) End(ss func(bool) error) error {
	if !ma.cpha1 {
		if _, err := ma.drv.Write([]byte{ma.cidle}); err != nil {
			return err
		}
	}
	if err := ma.drv.Flush(); err != nil {
		return err
	}
	if ss != nil {
		return ss(true)
	}
	return nil
}

// NoDelay can be used between frames to avoid produce delay bits (sometimes
// usefull if Config.Delay > 0). If used in middle of frame it desynchronizes
// stream of frames (subsequent delays will be inserted in wrong places).
func (ma *Master) NoDelay() {
	ma.rbif = 0
	ma.wbif = 0
}

// Write writes data to SPI bus. It divides data into frames and can generate
// delay bits between them. It uses driver's Write method with no more than 16
// bytes at once. Driver should implement buffering if such small chunks
// degrades performance.
func (ma *Master) Write(data []byte) (int, error) {
	var buf [16]byte
	mask := uint(0x80)
	if ma.lsbf {
		mask = uint(0x01)
	}
	for k, b := range data {
		if ma.delay != 0 {
			if ma.wbif == ma.flen {
				idle := []byte{ma.cidle, ma.cidle}
				for i := 0; i < ma.delay; i++ {
					if _, err := ma.drv.Write(idle); err != nil {
						return k, err
					}
				}
				ma.wbif = 0
			}
			ma.wbif++
		}
		u := uint(b)
		for i := 0; i < len(buf); i += 2 {
			out := ma.cfirst
			if mask&u != 0 {
				out |= ma.mosi
			}
			buf[i] = out
			buf[i+1] = out ^ ma.sclk
			if ma.lsbf {
				u >>= 1
			} else {
				u <<= 1
			}
		}
		if _, err := ma.drv.Write(buf[:]); err != nil {
			return k, err
		}
	}
	return len(data), nil
}

// WriteN writes b n times to SPI bus. See Write for more info.
func (ma *Master) WriteN(b byte, n int) (int, error) {
	var buf [16]byte
	mask := uint(0x80)
	if ma.lsbf {
		mask = uint(0x01)
	}
	u := uint(b)
	for i := 0; i < len(buf); i += 2 {
		out := ma.cfirst
		if mask&u != 0 {
			out |= ma.mosi
		}
		buf[i] = out
		buf[i+1] = out ^ ma.sclk
		if ma.lsbf {
			u >>= 1
		} else {
			u <<= 1
		}
	}
	for k := 0; k < n; k++ {
		if ma.delay != 0 {
			if ma.wbif == ma.flen {
				idle := []byte{ma.cidle, ma.cidle}
				for i := 0; i < ma.delay; i++ {
					if _, err := ma.drv.Write(idle); err != nil {
						return k, err
					}
				}
				ma.wbif = 0
			}
			ma.wbif++
		}
		if _, err := ma.drv.Write(buf[:]); err != nil {
			return k, err
		}
	}
	return n, nil
}

var ErrPhaseNoise = errors.New("noise or phase error")

func (ma *Master) Read(data []byte) (int, error) {
	if err := ma.drv.Flush(); err != nil {
		return 0, err
	}
	var buf [16]byte
	if ma.rbif != -1 {
		if n, err := ma.drv.Read(buf[:1]); n == 0 {
			if err == nil || err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, err
		}
		ma.rbif = 0
	}
	for k := range data {
		if ma.delay != 0 {
			if ma.rbif == ma.flen {
				for i := 0; i < ma.delay; i++ {
					if _, err := ma.drv.Read(buf[:2]); err != nil {
						return k, err
					}
				}
				ma.rbif = 0
			}
			ma.rbif++
		}
		if n, err := io.ReadFull(ma.drv, buf[:]); n < len(buf) {
			if err == io.EOF {
				err = nil
			}
			return k, err
		}
		var u uint
		for i := 0; i < len(buf); i += 2 {
			bit := buf[i]
			if bit != buf[i+1] {
				return k, ErrPhaseNoise
			}
			if ma.lsbf {
				u = u>>1 | 0x80
			} else {
				u = u<<1 | 0x01
			}
		}
		data[k] = byte(u)
	}
	return len(data), nil
}
