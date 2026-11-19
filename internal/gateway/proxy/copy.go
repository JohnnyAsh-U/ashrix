package proxy

import (
	"io"
	"sync"
)

const copyBufferSize = 32 * 1024

var copyBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, copyBufferSize)
		return &buf
	},
}

func copyResponse(dst io.Writer, src io.Reader) (int64, error) {
	buf := copyBufferPool.Get().(*[]byte)
	n, err := io.CopyBuffer(dst, src, *buf)
	copyBufferPool.Put(buf)
	return n, err
}
