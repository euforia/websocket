package websocket

import (
	"bytes"
	"compress/flate"
	"fmt"
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

// Sits between a flate writer and the underlying writer i.e. messageWriter
// Truncates last bytes of flate compresses message
type FlateAdaptor struct {
	last4bytes []byte

	last4Pos int

	msgWriter io.WriteCloser
}

func NewFlateAdaptor(w io.WriteCloser) *FlateAdaptor {
	fmt.Println("called")
	return &FlateAdaptor{msgWriter: w, last4bytes: []byte{}, last4Pos: 0}
}

/*
func (aw *FlateAdaptor) Write(p []byte) (n int, err error) {
	fmt.Println("Data size", len(p))
	fmt.Println("Curr last 4", aw.last4bytes)

	// Write the previous 4 bytes
	if aw.last4Pos == 4 {
		if _, err = aw.msgWriter.Write(aw.last4bytes[:]); err != nil {
			return
		}
		aw.last4Pos = 0
	}

	switch {
	case len(p) < (4 - aw.last4Pos):
		copy(aw.last4bytes[aw.last4Pos:], p)
		n = len(p)
		err = nil
		//aw.last4Pos =

		//aw.last4bytes[aw.last4Pos:],
		//case len(p)==4:
	}

	//copy(aw.last4bytes[aw.last4Pos:], p)
	//copied := 3 - aw.last4bytes
	//

	//fmt.Println("last 4 (after)", aw.last4bytes)

	// Write everything but the last 4 bytes
	//if len(p) > 4 {
	//fmt.Printf("writing to backend: %x\n", p[:len(p)-5])
	n, err = aw.msgWriter.Write(p[:len(p)-5])
	//}

	return
}
*/

func (aw *FlateAdaptor) Write(p []byte) (n int, err error) {
	//fmt.Printf("top writing: %d %x\n", len(p), p)

	//buffSpace := 4 - aw.last4Pos
	////
	t := append(aw.last4bytes, p...)
	fmt.Printf("t=%x\n", t)

	if len(t) > 4 {
		aw.last4bytes = make([]byte, 4)
		copy(aw.last4bytes, t[len(t)-4:])
		//aw.last4bytes = t[len(t)-4:]
		fmt.Printf("l=%x; writing (-4) : %x\n", aw.last4bytes, t[:len(t)-4])
		_, err = aw.msgWriter.Write(t[:len(t)-4])
	} else {
		aw.last4bytes = make([]byte, len(t))
		aw.last4bytes = t
		fmt.Printf("l=%x\n", aw.last4bytes)
	}
	////
	/*
		if len(p) > buffSpace {

			t := append(aw.last4bytes[:aw.last4Pos], p...)
			fmt.Printf("full: %x\n", t)

			copy(aw.last4bytes, t[len(t)-4:])

			fmt.Printf("last4 : %d - %x\n", len(aw.last4bytes), aw.last4bytes)

			fmt.Printf("writing (-4) : %x\n", t[:len(t)-4])
			if _, err = aw.msgWriter.Write(t[:len(t)-4]); err == nil {
				aw.last4Pos = 4
			}
		} else {

			copy(aw.last4bytes[aw.last4Pos:], p)
			fmt.Printf("%d - %v\n", len(aw.last4bytes), aw.last4bytes)

			aw.last4Pos += len(p)
		}
	*/

	n = len(p)

	//fmt.Printf("last4: %x\n", aw.last4bytes)
	fmt.Printf("l=%x\n", aw.last4bytes)
	return
}

func (aw *FlateAdaptor) writeEndBlock() (n int, err error) {
	// Only 1 byte to send if not ending in 0 and the rest it thown out.
	//fmt.Printf("size: %d\n", len(aws.))
	fmt.Printf("endblock %d: %x\n", len(aw.last4bytes), aw.last4bytes)

	if aw.last4bytes[3] != 0x00 {
		n, err = aw.msgWriter.Write([]byte{aw.last4bytes[0]})
	}
	//else {
	//n = 0, err = io.EOF
	//}

	return
}

func (aw *FlateAdaptor) Close() (err error) {
	if _, err = aw.writeEndBlock(); err == nil {
		err = aw.msgWriter.Close()
	}
	return
}

type FlateAdaptorWriter struct {
	flWriter  *flate.Writer
	flAdaptor *FlateAdaptor
}

func NewFlateAdaptorWriter(msgWriter io.WriteCloser, level int) (faw *FlateAdaptorWriter, err error) {
	faw = &FlateAdaptorWriter{
		flAdaptor: NewFlateAdaptor(msgWriter),
	}
	faw.flWriter, err = flate.NewWriter(faw.flAdaptor, level)
	return
}

func (faw *FlateAdaptorWriter) Write(p []byte) (int, error) {
	//fmt.Printf("String:%s\n", p)
	return faw.flWriter.Write(p)
}

func (faw *FlateAdaptorWriter) Close() (err error) {
	if err = faw.flWriter.Close(); err == nil {
		err = faw.flAdaptor.Close()
	}

	return
}

/*----------------------------------------------------------------------*/

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
	// Append 0 bit if not ending in a 0 bit
	var t []byte
	if fw.last5[4] != 0x00 {
		t = append(fw.last5, 0x00)
	}
	//t = t[:len(t)-5]
	// Truncate last 4 bytes
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
