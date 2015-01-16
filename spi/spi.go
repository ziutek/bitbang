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
	tord chan int8
	dr   bool
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
	dlyn   int
}

// New returns new SPI that uses r/w to read/write data using SPI protocol.
// Every byte that is read/written to r/w is treated as word of bits
// that are samples of SCLK, MOSI and MISO lines. For example
// byte&mosi != 0 means that MOSI line is high.
// New panics if sclk&mosi != 0. Default configuration is: MSBF, CPOL0,
// CPHA0, 8bit, no delay.
// TODO: Write about sampling rate.
func NewMaster(drv bitbang.SyncDriver, sclk, mosi, miso byte) *Master {
	if sclk&mosi != 0 {
		panic("scln&mosi!=0")
	}
	ma := new(Master)
	*ma = Master{
		drv:  drv,
		tord: make(chan int8, 4096/16), // Good value for 4 KiB write buf.
		sclk: sclk,
		mosi: mosi,
		miso: miso,
	}
	return ma
}

func (ma *Master) SetPrePost(pre, post []byte) {
	if len(pre) > 16 {
		panic("len(pre)>16")
	}
	if len(post) > 16 {
		panic("len(post)>16")
	}
	ma.pre = pre
	ma.post = post
}

func (ma *Master) PrePost() (pre, post []byte) {
	return ma.pre, ma.post
}

func (ma *Master) SetBase(base byte) {
	ma.base = base
}

func (ma *Master) Base() byte {
	return ma.base
}

// Configure configures SPI master. It can be used before start transaction to
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
		panic("Delay<0 || cfg.Delay>8")
	}
	if cfg.FrameLen <= 0 {
		panic("FrameLen <= 0")
	}
	ma.flen = -cfg.FrameLen
	ma.dlyn = cfg.Delay
}

// werror informs Read about Write error.
func (ma *Master) werror(err error) {
	ma.werr = err
	close(ma.tord)
	ma.tord = nil
	ma.wmtx.Unlock()
}

// toread informs read about data to read
func (ma *Master) toread(n int) error {
	if len(ma.tord) == cap(ma.tord) {
		// Read reads to slow. Probably drv write buffer is too big or
		// cap(ma.tord) too litle.
		if err := ma.drv.Flush(); err != nil {
			return err
		}
	}
	if n > 127 || n < -128 {
		panic("n>127 || n<-128")
	}
	ma.tord <- int8(n)
	return nil
}

// Begin should be called before starting transaction with slave device.
// In case of CPHA1 it sets SCLK line to its idle state.
func (ma *Master) Begin() error {
	ma.wmtx.Lock()
	if ma.werr != nil {
		ma.wmtx.Unlock()
		return ma.werr
	}
	ma.flen = -ma.flen
	ma.fn = 0
	n := len(ma.pre)
	if ma.cpha1 {
		n++
	}
	err := ma.toread(-n)
	if len(ma.pre) > 0 && err == nil {
		_, err = ma.drv.Write(ma.pre)
	}
	if ma.cpha1 && err == nil {
		_, err = ma.drv.Write([]byte{ma.base | ma.cidle})
	}
	if err != nil {
		ma.werror(err)
		return err
	}
	return nil
}

// Flush calls Driver.Flush.
func (ma *Master) Flush() error {
	if ma.werr != nil {
		ma.wmtx.Unlock()
		return ma.werr
	}
	err := ma.toread(0)
	if err == nil {
		err = ma.drv.Flush()
	}
	if err != nil {
		ma.werror(err)
	}
	return err
}

// End should be called after end of transaction with slave device. In case of
// CPHA1 it sets SCLK line to its idle state. After that it calls driver's
// Flush method to ensure that all bits are really sent.
func (ma *Master) End() error {
	if ma.werr != nil {
		ma.wmtx.Unlock()
		return ma.werr
	}
	ma.flen = -ma.flen
	n := len(ma.post)
	if !ma.cpha1 {
		n++
	}
	err := ma.toread(-n)
	if !ma.cpha1 && err == nil {
		_, err = ma.drv.Write([]byte{ma.base | ma.cidle})
	}
	if len(ma.post) > 0 && err == nil {
		_, err = ma.drv.Write(ma.post)
	}
	if err == nil {
		err = ma.toread(0)
	}
	if err == nil {
		err = ma.drv.Flush()
	}
	if err != nil {
		ma.werror(err)
		return err
	}
	ma.wmtx.Unlock()
	return nil
}

// NoDelay can be used between frames to avoid produce delay bits (sometimes
// usefull if Config.Delay > 0). If used in middle of frame it desynchronizes
// stream of frames (subsequent delays will be inserted in wrong places).
func (ma *Master) NoDelay() {
	ma.fn = 0
}

func (ma *Master) tobits(bits *[16]byte, b byte) {
	u := uint(b)
	mask := uint(0x80)
	if ma.lsbf {
		mask = 0x01
	}
	for i := 0; i < len(bits); i += 2 {
		bit := ma.base | ma.cfirst
		if mask&u != 0 {
			bit |= ma.mosi
		}
		bits[i] = bit
		bits[i+1] = bit ^ ma.sclk
		if ma.lsbf {
			u >>= 1
		} else {
			u <<= 1
		}
	}
}

func (ma *Master) writeBits(bits *[16]byte) error {
	if ma.dlyn > 0 {
		if ma.fn == ma.flen {
			idle := ma.base | ma.cidle
			ibuf := []byte{idle, idle}
			if err := ma.toread(-ma.dlyn * len(ibuf)); err != nil {
				return err
			}
			for i := 0; i < ma.dlyn; i++ {
				if _, err := ma.drv.Write(ibuf); err != nil {
					return err
				}
			}
			ma.fn = 0
		}
		ma.fn++
	}
	if err := ma.toread(len(bits)); err != nil {
		return err
	}
	_, err := ma.drv.Write(bits[:])
	return err
}

// Write writes data to SPI bus. It divides data into frames and generates
// delay bits between them if delay is configured. It uses driver's Write
// method with no more than 16 bytes at once. Driver should implement
// buffering if such small chunks degrades performance. Write causes
// len(data) bytes need to be read from SPI. Driver's buffers has usually
// limited size so the common idiom is to call Read and Write concurently
// to avoid blocking.
func (ma *Master) Write(data []byte) (int, error) {
	if ma.werr != nil {
		ma.wmtx.Unlock()
		return 0, ma.werr
	}
	if ma.flen < 0 {
		panic("Write outside Begin:End block")
	}
	var bits [16]byte
	for k, b := range data {
		ma.tobits(&bits, b)
		if err := ma.writeBits(&bits); err != nil {
			ma.werror(err)
			return k, err
		}
	}
	return len(data), nil
}

// WriteString works like Write..
func (ma *Master) WriteString(s string) (int, error) {
	if ma.werr != nil {
		ma.wmtx.Unlock()
		return 0, ma.werr
	}
	if ma.flen < 0 {
		panic("WriteString outside Begin:End block")
	}
	var bits [16]byte
	for k := 0; k < len(s); k++ {
		ma.tobits(&bits, s[k])
		if err := ma.writeBits(&bits); err != nil {
			ma.werror(err)
			return k, err
		}
	}
	return len(s), nil
}

// WriteN writes b n times to SPI bus. See Write for more info.
func (ma *Master) WriteN(b byte, n int) (int, error) {
	if ma.werr != nil {
		ma.wmtx.Unlock()
		return 0, ma.werr
	}
	if ma.flen < 0 {
		panic("WriteN outside Begin:End block")
	}
	var bits [16]byte
	ma.tobits(&bits, b)
	for k := 0; k < n; k++ {
		if err := ma.writeBits(&bits); err != nil {
			ma.werror(err)
			return k, err
		}
	}
	return n, nil
}

func (ma *Master) tobyte(bits *[16]byte) byte {
	var u uint
	if ma.lsbf {
		for i := 1; i < len(bits); i += 2 {
			u >>= 1
			if bits[i]&ma.miso != 0 {
				u |= 0x80
			}
		}
	} else {
		for i := 1; i < len(bits); i += 2 {
			u <<= 1
			if bits[i]&ma.miso != 0 {
				u |= 0x01
			}
		}
	}
	return byte(u)
}

func (ma *Master) discard(bits *[16]byte, ignfmark bool) error {
	if ma.dr {
		return nil
	}
	for {
		m, ok := <-ma.tord
		if !ok {
			return ma.werr
		}
		switch {
		case int(m) == len(bits):
			ma.dr = true
			return nil
		case m == 0:
			if ignfmark {
				continue
			}
			return nil
		case m > 0:
			panic("m>0 && m!=len(bits)")
		}
		if _, err := io.ReadFull(ma.drv, bits[:-m]); err != nil {
			return err
		}
	}
}

func (ma *Master) readBits(bits *[16]byte) error {
	n, err := io.ReadFull(ma.drv, bits[:])
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	} else if n != len(bits) {
		err = io.ErrUnexpectedEOF
	}
	ma.dr = false
	return err
}

// Read reads bytes from SPI bus into data. It always reads len(data) bytes or
// returns error (like io.ReadFull). If len(data)==0 Read only discards overhead
// bits up to the firs data bit or up the Flush mark.
func (ma *Master) Read(data []byte) (m int, err error) {
	var bits [16]byte
	if len(data) == 0 {
		err = ma.discard(&bits, false)
		return
	}
	for m < len(data) {
		if err = ma.discard(&bits, true); err != nil {
			return
		}
		if err = ma.readBits(&bits); err != nil {
			return
		}
		data[m] = ma.tobyte(&bits)
		m++
	}
	return
}

// ReadN reads and discards n bytes from SPI bus. If n==0 ReadN only discards
// overhead bits up to first data bit.
func (ma *Master) ReadN(n int) (m int, err error) {
	var bits [16]byte
	if n == 0 {
		err = ma.discard(&bits, false)
		return
	}
	for m < n {
		if err = ma.discard(&bits, true); err != nil {
			return
		}
		if err = ma.readBits(&bits); err != nil {
			return
		}
		m++
	}
	return
}
