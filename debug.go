package bitbang

import (
	"io"
)

type Debug struct {
	w io.Writer
}

func NewDebug(w io.Writer) Debug {
	return Debug{w: w}
}

func (d Debug) Write(data []byte) (int, error) {
	var out [16]byte
	for i := 1; i < 15; i += 2 {
		out[i] = '\t'
	}
	out[15] = '\n'

	for n, b := range data {
		mask := uint(0x80)
		for i := 0; i < 16; i += 2 {
			if mask&uint(b) == 0 {
				out[i] = '0'
			} else {
				out[i] = '1'
			}
			mask >>= 1
		}
		if _, err := d.w.Write(out[:]); err != nil {
			return n, err
		}
	}
	return len(data), nil
}

func (_ Debug) Read(data []byte) (int, error) {
	for i := range data {
		data[i] = 0
	}
	return len(data), nil
}
