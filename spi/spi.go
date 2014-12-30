// Package spi implements SPI protocol (http://en.wikipedia.org/wiki/Serial_Peripheral_Interface_Bus)
// using bit banging (http://en.wikipedia.org/wiki/Bit_banging).
package spi

import (
	"io"
	"sync"

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
	tord chan int
	werr error
	wmtx sync.Mutex
	fn   int
	pre  []byte
	post []byte
	sclk byte
	mosi byte
	miso byte
	base byte

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
	ma.init(drv, sclk, mosi, miso)
	return ma
}

func (ma *Master) init(drv bitbang.SyncDriver, sclk, mosi, miso byte) {
	if sclk&mosi != 0 {
		panic("spi.Master.init: scln&mosi != 0")
	}
	*ma = Master{
		drv:  drv,
		tord: make(chan int, 16),
		sclk: sclk,
		mosi: mosi,
		miso: miso,
	}
}

func (ma *Master) SetPrePost(pre, post []byte) {
	ma.pre = pre
	ma.post = post
}

func (ma *Master) SetBase(base byte) {
	ma.base = base
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
	if cfg.Delay < 0 || cfg.Delay > 8 {
		panic("Delay < 0 || cfg.Delay > 8")
	}
	if cfg.FrameLen <= 0 {
		panic("FrameLen <= 0")
	}
	ma.flen = -cfg.FrameLen
	ma.delay = cfg.Delay
}

// werror informs Read about Write error.
func (ma *Master) werror(err error) {
	ma.werr = err
	close(ma.tord)
}

// Begin should be called before starting conversation with slave device.
// In case of CPHA1 it sets SCLK line to its idle state.
func (ma *Master) Begin() error {
	ma.wmtx.Lock()
	ma.flen = -ma.flen
	ma.fn = 0
	var (
		m, n int
		err  error
	)
	if len(ma.pre) > 0 {
		n, err = ma.drv.Write(ma.pre)
	}
	if ma.cpha1 && err == nil {
		m, err = ma.drv.Write([]byte{ma.cidle})
		n += m
	}
	if n != 0 {
		ma.tord <- -n
	}
	if err != nil {
		ma.werror(err)
		ma.wmtx.Unlock()
	}
	return err
}

// End should be called after end of conversation with slave device. In case of
// CPHA1 it sets SCLK line to its idle state. After that it calls driver's
// Flush method to ensure that all bits are really sent.
func (ma *Master) End() error {
	ma.flen = -ma.flen
	var (
		m, n int
		err  error
	)
	if !ma.cpha1 {
		n, err = ma.drv.Write([]byte{ma.cidle})
	}
	if len(ma.post) > 0 && err == nil {
		m, err = ma.drv.Write(ma.post)
		n += m
	}
	if n != 0 {
		ma.tord <- -n
	}
	if err == nil {
		err = ma.drv.Flush()
	}
	if err != nil {
		ma.werror(err)
	}
	ma.wmtx.Unlock()
	return err
}

// NoDelay can be used between frames to avoid produce delay bits (sometimes
// usefull if Config.Delay > 0). If used in middle of frame it desynchronizes
// stream of frames (subsequent delays will be inserted in wrong places).
func (ma *Master) NoDelay() {
	ma.fn = 0
}

// Write writes data to SPI bus. It divides data into frames and generates
// delay bits between them if delay is configured. It uses driver's Write
// method with no more than 16 bytes at once. Driver should implement
// buffering if such small chunks degrades performance. Write causes
// len(data) bytes need to be read from SPI. Driver's buffers has usually
// limited size so the common idiom is to call Read and Write concurently
// to avoid blocking.
func (ma *Master) Write(data []byte) (int, error) {
	if ma.flen < 0 {
		panic("Write outside Begin:End block")
	}
	var buf [16]byte
	mask := uint(0x80)
	if ma.lsbf {
		mask = uint(0x01)
	}
	for k, b := range data {
		if ma.delay > 0 {
			if ma.fn == ma.flen {
				idle := ma.base | ma.cidle
				ibuf := []byte{idle, idle}
				var (
					n, m int
					err  error
				)
				for i := 0; i < ma.delay && err == nil; i++ {
					m, err = ma.drv.Write(ibuf)
					n -= m

				}
				if n != 0 {
					ma.tord <- n
				}
				if err != nil {
					ma.werror(err)
					return k, err
				}
				ma.fn = 0
			}
			ma.fn++
		}
		u := uint(b)
		for i := 0; i < len(buf); i += 2 {
			out := ma.base | ma.cfirst
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
		n, err := ma.drv.Write(buf[:])
		if n != 0 {
			ma.tord <- n
		}
		if err != nil {
			ma.werror(err)
			return k, err
		}
	}
	return len(data), nil
}

// WriteN writes b n times to SPI bus. See Write for more info.
func (ma *Master) WriteN(b byte, n int) (int, error) {
	if ma.flen < 0 {
		panic("WriteN outside Begin:End block")
	}
	var buf [16]byte
	mask := uint(0x80)
	if ma.lsbf {
		mask = uint(0x01)
	}
	u := uint(b)
	for i := 0; i < len(buf); i += 2 {
		out := ma.base | ma.cfirst
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
		if ma.delay > 0 {
			if ma.fn == ma.flen {
				idle := ma.base | ma.cidle
				ibuf := []byte{idle, idle}
				var (
					m, n int
					err  error
				)
				for i := 0; i < ma.delay && err == nil; i++ {
					m, err = ma.drv.Write(ibuf)
					n -= m
				}
				if n != 0 {
					ma.tord <- n
				}
				if err != nil {
					ma.werror(err)
					return k, err
				}
				ma.fn = 0
			}
			ma.fn++
		}
		n, err := ma.drv.Write(buf[:])
		if n != 0 {
			ma.tord <- n
		}
		if err != nil {
			ma.werror(err)
			return k, err
		}
	}
	return n, nil
}

// Flush wraps Driver.Flush method.
func (ma *Master) Flush() error {
	err := ma.drv.Flush()
	if err != nil {
		ma.werror(err)
	}
	return err
}

// Read reads bytes from SPI bus into data. It always reads len(data) bytes or
// returns error (like io.ReadFull).
func (ma *Master) Read(data []byte) (int, error) {
	var buf [16]byte
	for k := range data {
		var m int
		for {
			var ok bool
			m, ok = <-ma.tord
			if !ok {
				return k, ma.werr
			}
			if m > 0 {
				break
			}
			if _, err := io.ReadFull(ma.drv, buf[:-m]); err != nil {
				return k, err
			}
		}
		if i, err := io.ReadFull(ma.drv, buf[:m]); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return k, err
		} else if i != len(buf) {
			return k, io.ErrUnexpectedEOF
		}
		var u uint
		if ma.lsbf {
			for i := 1; i < len(buf); i += 2 {
				u >>= 1
				if buf[i]&ma.miso != 0 {
					u |= 0x80
				}
			}
		} else {
			for i := 1; i < len(buf); i += 2 {
				u <<= 1
				if buf[i]&ma.miso != 0 {
					u |= 0x01
				}
			}
		}
		data[k] = byte(u)
	}
	return len(data), nil
}

// ReadN reads and discards n bytes from SPI bus.
func (ma *Master) ReadN(n int) (int, error) {
	var buf [16]byte
	for k := 0; k < n; k++ {
		var m int
		for {
			var ok bool
			m, ok = <-ma.tord
			if !ok {
				return k, ma.werr
			}
			if m >= 0 {
				break
			}
			if _, err := io.ReadFull(ma.drv, buf[:-m]); err != nil {
				return k, err
			}
		}
		if i, err := io.ReadFull(ma.drv, buf[:m]); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return k, err
		} else if i != len(buf) {
			return k, io.ErrUnexpectedEOF
		}
	}
	return n, nil
}

func (ma *Master) write(out []byte, extend int) {
	if ma.Begin() != nil {
		return
	}
	if extend < 0 {
		var b byte
		if len(out) > 0 {
			b = out[0]
		}
		if _, err := ma.WriteN(b, -extend); err != nil {
			return
		}
	}
	if _, err := ma.Write(out); err != nil {
		return
	}
	if extend > 0 {
		var b byte
		if len(out) > 0 {
			b = out[len(out)-1]
		}
		if _, err := ma.WriteN(b, extend); err != nil {
			return
		}
	}
	ma.End()
	return
}

func (ma *Master) read(in []byte, discard int) (n int, err error) {
	if discard < 0 {
		if _, err = ma.ReadN(-discard); err != nil {
			return
		}
	}
	if n, err = ma.Read(in); err != nil {
		return
	}
	if discard > 0 {
		_, err = ma.ReadN(discard)
	}
	return

}

// WriteRead performs full SPI conversation (calls Begin, writes data from out
// and calls End concurrently with reading data into in).
// If len(in) < len(out) it discards last len(out)-len(in) read bytes.
// if len(in) > len(out) it extends out repeat last out byte len(in)-len(out)
// times. If len(out) == 0 it writes 0 byte len(in) times.
// If err==nil then in is fully filled. If err!=nil only first n bytes was read.
func (ma *Master) WriteRead(out, in []byte) (n int, err error) {
	var discard int
	extend := len(in) - len(out)
	if extend < 0 {
		discard = -extend
		extend = 0
	}
	go ma.write(out, extend)
	n, err = ma.read(in, discard)
	return
}
