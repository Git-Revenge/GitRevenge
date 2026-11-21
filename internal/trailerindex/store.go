// Package trailerindex keeps every commit's trailers, read once from a repository's
// history and stored outside it, so that asking which commits carry a trailer never reads
// the history again.
//
// A commit never changes, so its trailers never do: the index only grows. It is a cache.
// Deleting its file is always safe, and nothing is ever written into the repository.
package trailerindex

import (
	"bufio"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
)

// The file is a short header and then frames, each appended in one write and never changed.
//
//	header: "GRTIDX\x00", format version, parser version (2 bytes), id size
//	frame:  magic (4 bytes), kind, payload length (4 bytes), CRC-32 of the payload, payload
//
// Three kinds of frame tell the story of a read of the history, called a pass:
//
//	begin     the pass's number, and the commits it reads from (tips) and stops at (not)
//	batch     the pass's number, how many commits it has walked so far, and the trailers
//	          of those among them that have any
//	complete  the pass's number: everything reachable from its tips is in the file
//
// A batch carries its own strings, numbered within the frame, so a frame means the same
// whatever came before it. That matters because two copies of the app can have the same
// repository open: each appends whole frames, and neither has to know what the other
// wrote. A commit both read is in the file twice and counted once when it's loaded.
//
// A frame cut short by a crash fails its checksum. The reader steps over it to the next
// frame mark, so what was appended later is still read, and nothing is ever truncated.
const (
	fileMagic     = "GRTIDX\x00"
	formatVersion = 1
	headerSize    = len(fileMagic) + 4

	frameHeaderSize = 13
	// maxFrame bounds a frame's length, so a damaged length can't ask for the whole disk.
	maxFrame = 1 << 30
)

// frameMagic starts every frame: "GRTF", read as a little-endian number.
const frameMagic uint32 = 0x46545247

// The kinds of frame.
const (
	frameBegin    byte = 1
	frameBatch    byte = 2
	frameComplete byte = 3
)

// errStale means the file was written for another parser, format or kind of object id,
// and has to be read again from the history.
var errStale = errors.New("the trailer index was written by another version")

func encodeHeader(parser uint16, idSize int) []byte {
	b := make([]byte, 0, headerSize)
	b = append(b, fileMagic...)
	b = append(b, formatVersion)
	b = binary.LittleEndian.AppendUint16(b, parser)
	return append(b, byte(idSize))
}

// decodeHeader returns the id size the file was written with, or errStale.
func decodeHeader(b []byte, parser uint16) (idSize int, err error) {
	if len(b) < headerSize || string(b[:len(fileMagic)]) != fileMagic || b[len(fileMagic)] != formatVersion {
		return 0, errStale
	}
	if binary.LittleEndian.Uint16(b[len(fileMagic)+1:]) != parser {
		return 0, errStale
	}
	idSize = int(b[len(fileMagic)+3])
	if idSize != 20 && idSize != 32 {
		return 0, errStale
	}
	return idSize, nil
}

func encodeFrame(kind byte, payload []byte) []byte {
	b := make([]byte, 0, frameHeaderSize+len(payload))
	b = binary.LittleEndian.AppendUint32(b, frameMagic)
	b = append(b, kind)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(payload)))
	b = binary.LittleEndian.AppendUint32(b, crc32.ChecksumIEEE(payload))
	return append(b, payload...)
}

// eachFrame calls fn with every whole frame of f after the header, in order, stepping
// over anything that isn't one.
func eachFrame(f io.ReaderAt, size int64, fn func(kind byte, payload []byte) error) error {
	at := int64(headerSize)
	for at+frameHeaderSize <= size {
		kind, payload, next, ok := frameAt(f, size, at)
		if !ok {
			// Not a frame here: look for the next mark, a byte at a time.
			found, err := nextMark(f, size, at+1)
			if err != nil {
				return err
			}
			if found < 0 {
				return nil
			}
			at = found
			continue
		}
		if err := fn(kind, payload); err != nil {
			return err
		}
		at = next
	}
	return nil
}

// frameAt reads the frame that starts at at, if a whole and undamaged one does.
func frameAt(f io.ReaderAt, size, at int64) (kind byte, payload []byte, next int64, ok bool) {
	var head [frameHeaderSize]byte
	if _, err := f.ReadAt(head[:], at); err != nil {
		return 0, nil, 0, false
	}
	if binary.LittleEndian.Uint32(head[:]) != frameMagic {
		return 0, nil, 0, false
	}
	n := int64(binary.LittleEndian.Uint32(head[5:]))
	if n > maxFrame || at+frameHeaderSize+n > size {
		return 0, nil, 0, false
	}
	payload = make([]byte, n)
	if _, err := f.ReadAt(payload, at+frameHeaderSize); err != nil {
		return 0, nil, 0, false
	}
	if crc32.ChecksumIEEE(payload) != binary.LittleEndian.Uint32(head[9:]) {
		return 0, nil, 0, false
	}
	return head[4], payload, at + frameHeaderSize + n, true
}

// nextMark returns where the next frame mark is at or after from, or -1.
func nextMark(f io.ReaderAt, size, from int64) (int64, error) {
	if from >= size {
		return -1, nil
	}
	br := bufio.NewReaderSize(io.NewSectionReader(f, from, size-from), 1<<16)
	var window uint32
	for i := int64(0); ; i++ {
		c, err := br.ReadByte()
		if errors.Is(err, io.EOF) {
			return -1, nil
		}
		if err != nil {
			return -1, err
		}
		// The mark is stored little-endian, so its first byte is the lowest.
		window = window>>8 | uint32(c)<<24
		if i >= 3 && window == frameMagic {
			return from + i - 3, nil
		}
	}
}

// appendFrames adds frames to the file at path in one write, creating it with its header
// if it isn't there. A file another version wrote must be removed first.
func appendFrames(path string, parser uint16, idSize int, frames []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err == nil && st.Size() == 0 {
		// The header goes in the same write as the first frames. If two copies of the app
		// both start the file, the second header lands between frames and is stepped over.
		frames = append(encodeHeader(parser, idSize), frames...)
	}
	if err == nil {
		_, err = f.Write(frames)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// A reader over a frame's payload. Anything short or out of range sets bad, and every
// read after that gives zero, so a payload is checked once at the end.
type decoder struct {
	b   []byte
	bad bool
}

func (d *decoder) uvarint() uint64 {
	v, n := binary.Uvarint(d.b)
	if n <= 0 {
		d.bad = true
		d.b = nil
		return 0
	}
	d.b = d.b[n:]
	return v
}

// count reads a length, which can't be more than what's left to read.
func (d *decoder) count() int {
	v := d.uvarint()
	if v > uint64(len(d.b)) {
		d.bad = true
		d.b = nil
		return 0
	}
	return int(v)
}

func (d *decoder) uint64() uint64 {
	if len(d.b) < 8 {
		d.bad = true
		d.b = nil
		return 0
	}
	v := binary.LittleEndian.Uint64(d.b)
	d.b = d.b[8:]
	return v
}

func (d *decoder) bytes(n int) []byte {
	if n < 0 || len(d.b) < n {
		d.bad = true
		d.b = nil
		return nil
	}
	v := d.b[:n]
	d.b = d.b[n:]
	return v
}

// A pass's begin frame.
type passBegin struct {
	nonce     uint64
	tips, not [][]byte
}

func encodeBegin(p passBegin) []byte {
	b := binary.LittleEndian.AppendUint64(nil, p.nonce)
	for _, set := range [][][]byte{p.tips, p.not} {
		b = binary.AppendUvarint(b, uint64(len(set)))
		for _, id := range set {
			b = append(b, id...)
		}
	}
	return encodeFrame(frameBegin, b)
}

func decodeBegin(payload []byte, idSize int) (passBegin, bool) {
	d := decoder{b: payload}
	p := passBegin{nonce: d.uint64()}
	for _, set := range []*[][]byte{&p.tips, &p.not} {
		n := d.count()
		for range n {
			*set = append(*set, d.bytes(idSize))
		}
	}
	return p, !d.bad
}

// One commit's trailers in a batch: its id, and for each trailer the numbers of its key
// and its value among the batch's strings.
type batchCommit struct {
	id    []byte
	pairs []uint32
}

// A pass's batch frame.
type passBatch struct {
	nonce uint64
	// done is how many commits the pass has walked once this batch is in, with or without
	// trailers: where to pick the walk up again.
	done    int
	strs    []string
	commits []batchCommit
}

func encodeBatch(p passBatch) []byte {
	b := binary.LittleEndian.AppendUint64(nil, p.nonce)
	b = binary.AppendUvarint(b, uint64(p.done))
	b = binary.AppendUvarint(b, uint64(len(p.strs)))
	for _, s := range p.strs {
		b = binary.AppendUvarint(b, uint64(len(s)))
		b = append(b, s...)
	}
	b = binary.AppendUvarint(b, uint64(len(p.commits)))
	for _, c := range p.commits {
		b = append(b, c.id...)
		b = binary.AppendUvarint(b, uint64(len(c.pairs)/2))
		for _, v := range c.pairs {
			b = binary.AppendUvarint(b, uint64(v))
		}
	}
	return encodeFrame(frameBatch, b)
}

// decodeBatch reads a batch frame. Its strings share one allocation.
func decodeBatch(payload []byte, idSize int) (passBatch, bool) {
	d := decoder{b: payload}
	p := passBatch{nonce: d.uint64(), done: int(d.uvarint())}
	n := d.count()
	// The strings are copied out together and cut up, so a batch costs one allocation for
	// them and the frame's bytes can be let go.
	type span struct{ from, to int }
	spans := make([]span, 0, n)
	var all []byte
	for range n {
		s := d.bytes(d.count())
		spans = append(spans, span{len(all), len(all) + len(s)})
		all = append(all, s...)
	}
	text := string(all)
	p.strs = make([]string, len(spans))
	for i, sp := range spans {
		p.strs[i] = text[sp.from:sp.to]
	}
	commits := d.count()
	p.commits = make([]batchCommit, 0, commits)
	for range commits {
		c := batchCommit{id: d.bytes(idSize)}
		pairs := d.count()
		c.pairs = make([]uint32, 0, pairs*2)
		for range pairs * 2 {
			v := d.uvarint()
			if v >= uint64(len(p.strs)) {
				d.bad = true
			}
			c.pairs = append(c.pairs, uint32(v))
		}
		p.commits = append(p.commits, c)
	}
	return p, !d.bad
}

func encodeComplete(nonce uint64) []byte {
	return encodeFrame(frameComplete, binary.LittleEndian.AppendUint64(nil, nonce))
}

func decodeComplete(payload []byte) (uint64, bool) {
	d := decoder{b: payload}
	nonce := d.uint64()
	return nonce, !d.bad
}
