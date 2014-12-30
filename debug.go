package bitbang

import (
	"io"
	"strconv"
)

type Debug struct {
	w io.Writer
}

func (_ Debug) Read(_ []byte) (int, error) { return 0, nil }
func (_ Debug) Flush() error               { return nil }

func NewDebug(w io.Writer) Debug {
	return Debug{w: w}
}

func (d Debug) Write(data []byte) (int, error) {
	var out [19]byte
	for i := 1; i < 15; i += 2 {
		out[i] = '\t'
	}
	out[15] = '\t'
	out[18] = '\n'

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
		out[17] = 0
		strconv.AppendUint(out[16:16:18], uint64(b), 16)
		if out[17] == 0 {
			out[17] = out[16]
			out[16] = '0'
		}
		if _, err := d.w.Write(out[:]); err != nil {
			return n, err
		}
	}
	return len(data), nil
}
