package main

import (
	"fmt"
	"os"

	"github.com/ziutek/bitbang"
	"github.com/ziutek/bitbang/spi"
)

func checkErr(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func main() {
	d := bitbang.NewDebug(os.Stdout)
	s := spi.New(nil, d, 0x01, 0x10, 0)
	n, err := s.Write([]byte{0x55, 0xaa})
	fmt.Println("--")
	fmt.Println(n, "bytes written")
	checkErr(err)
}
