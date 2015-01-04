package spi

import (
	"bytes"
	"testing"
)

type testdrv struct {
	*bytes.Buffer
}

func (_ testdrv) Flush() error { return nil }

type wtest struct {
	cfg Config
	in  []byte
	out []byte
}

func (wt *wtest) check(t *testing.T) {
	drv := testdrv{bytes.NewBuffer(make([]byte, 0, len(wt.out)))}
	ma := NewMaster(drv, 0x01, 0x10, 0)
	ma.SetPrePost([]byte{0x80}, []byte{0x80})
	ma.Configure(wt.cfg)
	ma.Begin()
	ma.Write(wt.in)
	ma.End()
	out := drv.Bytes()
	if bytes.Equal(wt.out, out) {
		return
	}
	t.Errorf(
		"\n%+v\nin=%#v\ngood=%#v\nout =%#v\n\n",
		wt.cfg, wt.in, wt.out, out,
	)
}

var wts = []wtest{
	{
		cfg: Config{MSBF | CPOL0 | CPHA0, 1, 0},
		in:  []byte{0x55, 0xaa},
		out: []byte{
			0x80,

			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,
			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,

			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,
			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,

			0x00,
			0x80,
		},
	},
	{
		cfg: Config{MSBF | CPOL1 | CPHA1, 1, 0},
		in:  []byte{0x55, 0xaa},
		out: []byte{
			0x80,
			0x01,

			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,
			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,

			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,
			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,

			0x80,
		},
	},
	{
		cfg: Config{LSBF | CPOL0 | CPHA0, 1, 0},
		in:  []byte{0xf0, 0x0f},
		out: []byte{
			0x80,

			0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01,
			0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11,

			0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11,
			0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01,

			0x00,
			0x80,
		},
	},
	{
		cfg: Config{LSBF | CPOL1 | CPHA0, 1, 0},
		in:  []byte{0xf0, 0x0f},
		out: []byte{
			0x80,

			0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00,
			0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10,

			0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10,
			0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00,

			0x01,
			0x80,
		},
	},
	{
		cfg: Config{LSBF | CPOL0 | CPHA1, 1, 0},
		in:  []byte{0xf0, 0x0f},
		out: []byte{
			0x80,
			0x00,

			0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00,
			0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10,

			0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10,
			0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00,

			0x80,
		},
	},
	{
		cfg: Config{LSBF | CPOL1 | CPHA1, 1, 0},
		in:  []byte{0xf0, 0x0f},
		out: []byte{
			0x80,
			0x01,

			0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01,
			0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11,

			0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11,
			0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01,

			0x80,
		},
	},
	{
		cfg: Config{MSBF | CPOL0 | CPHA0, 1, 1},
		in:  []byte{0x55, 0xaa, 0xf0, 0x0f},
		out: []byte{
			0x80,

			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,
			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,

			0x00, 0x00,

			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,
			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,

			0x00, 0x00,

			0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11,
			0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01,

			0x00, 0x00,

			0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01,
			0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11,

			0x00,
			0x80,
		},
	},
	{
		cfg: Config{MSBF | CPOL0 | CPHA0, 2, 2},
		in:  []byte{0x55, 0xaa, 0xf0, 0x0f},
		out: []byte{
			0x80,

			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,
			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,

			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,
			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,

			0x00, 0x00, 0x00, 0x00,

			0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11,
			0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01,

			0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01,
			0x10, 0x11, 0x10, 0x11, 0x10, 0x11, 0x10, 0x11,

			0x00,
			0x80,
		},
	},
	{
		cfg: Config{MSBF | CPOL0 | CPHA0, 1, 0},
		in:  nil,
		out: []byte{
			0x80,

			0x00,
			0x80,
		},
	},	
}

func TestWrite(t *testing.T) {
	for _, wt := range wts {
		wt.check(t)
	}
}

type wntest struct {
	cfg Config
	b   byte
	n   int
	out []byte
}

func (wnt *wntest) check(t *testing.T) {
	drv := testdrv{bytes.NewBuffer(make([]byte, 0, len(wnt.out)))}
	ma := NewMaster(drv, 0x01, 0x10, 0)
	ma.SetPrePost([]byte{0x80}, []byte{0x80})
	ma.Configure(wnt.cfg)
	ma.Begin()
	ma.WriteN(wnt.b, wnt.n)
	ma.End()
	out := drv.Bytes()
	if bytes.Equal(wnt.out, out) {
		return
	}
	t.Errorf(
		"\n%+v\nb=%x n=%d\ngood=%#v\nout =%#v\n\n",
		wnt.cfg, wnt.b, wnt.n, wnt.out, out,
	)
}

var wnts = []wntest{
	{
		cfg: Config{MSBF | CPOL0 | CPHA0, 1, 0},
		b:   0x55,
		n:   2,
		out: []byte{
			0x80,

			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,
			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,

			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,
			0x00, 0x01, 0x10, 0x11, 0x00, 0x01, 0x10, 0x11,

			0x00,
			0x80,
		},
	},
	{
		cfg: Config{MSBF | CPOL0 | CPHA0, 1, 0},
		b:   0xaa,
		n:   2,
		out: []byte{
			0x80,

			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,
			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,

			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,
			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,

			0x00,
			0x80,
		},
	},
	{
		cfg: Config{MSBF | CPOL0 | CPHA0, 1, 1},
		b:   0xaa,
		n:   2,
		out: []byte{
			0x80,

			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,
			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,

			0x00, 0x00,

			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,
			0x10, 0x11, 0x00, 0x01, 0x10, 0x11, 0x00, 0x01,

			0x00,
			0x80,
		},
	},
}

func TestWriteN(t *testing.T) {
	for _, wnt := range wnts {
		wnt.check(t)
	}
}
