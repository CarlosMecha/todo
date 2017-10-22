package testutil

import "bytes"

// BufferCloser is a buffer implementation with Close operation
// Is a io.ReadCloser
type BufferCloser struct {
	b *bytes.Buffer
}

// NewBufferCloser returns the buffer with the data stored.
func NewBufferCloser(data []byte) *BufferCloser {
	return &BufferCloser{bytes.NewBuffer(data)}
}

func (b *BufferCloser) Read(p []byte) (n int, err error) {
	return b.b.Read(p)
}

func (b *BufferCloser) Close() error {
	return nil
}

func (b *BufferCloser) Get() []byte {
	return b.b.Bytes()
}
