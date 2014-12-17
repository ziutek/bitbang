package bitbang
 
// SyncDriver should implement sync bit banging, eg: for any byte written it
// provides one byte to read.
type SyncDriver interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	PurgeReadBuffer() error
	Flush() error
}