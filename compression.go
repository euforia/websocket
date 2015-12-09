package websocket

import (
	"bytes"
	"compress/flate"
	"io"
)

const (
	// Supported compression algorithm and parameters.
	CompressPermessageDeflate = "permessage-deflate; server_no_context_takeover; client_no_context_takeover"

	// Deflate compression level
	compressDeflateLevel int = 3

	// Deflate buffer size
	compressDeflateBufferSize = 1024
)

type FlateWriter struct {
	flt *flate.Writer

	// Underlying writer (ie. messageWriter)
	msgWriter io.WriteCloser

	// Buffer holding compressed data.
	buff *bytes.Buffer

	// Last 5 bytes of compressed data after each write.  This is needed
	// so the end block logic can be applied per RFC.
	last5 []byte
}

//func NewFlateWriter(w messageWriter) (fw *FlateWriter, err error) {
func NewFlateWriter(w io.WriteCloser) (fw *FlateWriter, err error) {
	fw = &FlateWriter{
		msgWriter: w,
		buff:      new(bytes.Buffer),
	}

	fw.flt, err = flate.NewWriter(fw.buff, compressDeflateLevel)

	return
}

func (fw *FlateWriter) write(final bool, b []byte) (n int, err error) {
	//fmt.Printf("orig size: %s\n", b)
	if n, err = fw.flt.Write(b); err != nil {
		return
	}
	if err = fw.flt.Flush(); err != nil {
		return
	}
	//fmt.Printf("written: %d; buff: %d\n", n, fw.buff.Len())
	buffBytes := make([]byte, fw.buff.Len())

	var rn int
	if rn, err = fw.buff.Read(buffBytes); err != nil {
		return
	}
	// Last 5 compressed bytes
	if len(fw.last5) == 0 {
		//fmt.Printf("Writing: %d\n", len(buffBytes[:rn-5]))
		_, err = fw.msgWriter.Write(buffBytes[:rn-5])
	} else {
		//fmt.Printf("Writing: %d\n", len(fw.last5)+len(buffBytes[:rn-5]))
		_, err = fw.msgWriter.Write(append(fw.last5, buffBytes[:rn-5]...))
	}
	if err != nil {
		return
	}

	fw.last5 = buffBytes[rn-5 : rn]
	//fmt.Printf("Last5: %d %x (%v)\n", len(fw.last5), fw.last5, final)
	if final {
		_, err = fw.writeEndBlock()
	}
	//fmt.Println(err, n)
	return

}

func (fw *FlateWriter) ReadFrom(r io.Reader) (nn int64, err error) {
	var (
		buff       = make([]byte, compressDeflateBufferSize)
		n          int        // read per loop
		totalWrote = int64(0) // total compressed bytes
	)

	nn = 0

	for {
		n, err = r.Read(buff)
		nn += int64(n)

		if err != nil {
			if err == io.EOF {
				err = nil
			}
			//fmt.Printf("[ReadFrom: readBytes: %d; compressedBytes: %d;]\n", nn, totalWrote)
			return
		}

		var fi int
		fi, err = fw.write(false, buff[:n])
		totalWrote += int64(fi)
		if err != nil {
			return
		}
	}

	return
}

func (fw *FlateWriter) Write(b []byte) (n int, err error) {
	return fw.write(false, b)
}

func (fw *FlateWriter) writeEndBlock() (n int, err error) {
	// Append 0 bit and truncate last 4 bytes
	var t []byte
	if fw.last5[4] != 0x00 {
		t = append(fw.last5, 0x00)
	}
	//t = t[:len(t)-5]
	n, err = fw.msgWriter.Write(t[:len(t)-5])
	return
}

func (fw *FlateWriter) Close() (err error) {
	if _, err = fw.writeEndBlock(); err != nil {
		return err
	}
	if err = fw.flt.Close(); err != nil {
		return
	}
	return fw.msgWriter.Close()
}
