package spi

func (ma *Master) write(oi [][]byte) {
	if ma.Begin() != nil {
		return
	}
	for k := 0; k < len(oi); k += 2 {
		out := oi[k]
		if len(out) > 0 {
			if _, err := ma.Write(out); err != nil {
				return
			}
		}
		ilen := 0
		if k+1 < len(oi) {
			ilen = len(oi[k+1])
		}
		if extend := ilen - len(out); extend > 0 {
			var b byte
			if len(out) > 0 {
				b = out[len(out)-1]
			}
			if _, err := ma.WriteN(b, extend); err != nil {
				return
			}
		}
	}
	ma.End()
	return
}

func (ma *Master) read(oi [][]byte) (n int, err error) {
	// Read bytes into os[2*k+1].
	for k := 0; k < len(oi); k += 2 {
		var in []byte
		if k+1 < len(oi) {
			in = oi[k+1]
		}
		if len(in) > 0 {
			if n, err = ma.Read(in); err != nil {
				return
			}
		}
		if discard := len(oi[k]) - len(in); discard > 0 {
			if _, err = ma.ReadN(discard); err != nil {
				return
			}
		}
	}
	// Discard overhead bits produced by End.
	return ma.Read(nil)
}

// WriteRead performs full SPI transaction using oi output/input buffers.
//
// For 0 <= k <= len(oi)/2 it treats oi[2*k] as out[k] and oi[2*k+1] as in[k].
// WriteRead performs following operations:
// 1. Calls ma.Begin(),
// 2. Writes provided data from out[k] buffers,
// 3. Calls ma.End(),
// concurrently with reading data into in[k] buffers.
//
// If len(in[k]) < len(out[k]) it discards last len(out[k])-len(in[k)) read
// bytes. if len(in[k]) > len(out[k]) it extends out[k] by repeating its last
// byte len(in[k])-len(out[k]) times. If len(out[k]) == 0 it writes 0 byte
// len(in[k]) times.
//
// After return, n contains number of bytes read into input buffers. err == nil
// menas that all input buffers arr fully filed. if err != nil then only first
// n bytes were read into input buffers.
func (ma *Master) WriteRead(oi ...[]byte) (n int, err error) {
	go ma.write(oi)
	return ma.read(oi)
}
