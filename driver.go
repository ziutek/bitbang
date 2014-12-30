package bitbang
 
// SyncDriver should implement sync bit banging, eg: for any byte written it
// provides one byte to read. SyncDriver must ensure that Read returns error
// if previous Write returned error.
type SyncDriver interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Flush() error
}