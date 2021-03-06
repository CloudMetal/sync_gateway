package channels

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	// "log"
)

// Encodes a ChangeLog into a simple appendable data format.
func (ch *ChangeLog) Encode(w io.Writer) {
	writeSequence(ch.Since, w)
	for _, entry := range ch.Entries {
		entry.Encode(w, "")
	}
}

// Encodes a LogEntry in a format that can be appended to an encoded ChangeLog.
func (entry *LogEntry) Encode(w io.Writer, parent string) {
	binary.Write(w, binary.BigEndian, entry.Flags)
	writeSequence(entry.Sequence, w)
	writeString(entry.DocID, w)
	writeString(entry.RevID, w)
	writeString(parent, w)
}

// Decodes an encoded ChangeLog.
func DecodeChangeLog(r *bytes.Reader) *ChangeLog {
	type docAndRev struct {
		docID, revID string
	}

	ch := ChangeLog{
		Since:   readSequence(r),
		Entries: make([]*LogEntry, 0, 500),
	}
	parents := map[docAndRev]*LogEntry{}
	var buf [1]byte
	for {
		n, _ := r.Read(buf[0:1])
		if n == 0 {
			break // eof
		}
		if buf[0] > 7 {
			panic("bad flags")
		}
		entry := &LogEntry{
			Flags:    buf[0],
			Sequence: readSequence(r),
			DocID:    readString(r),
			RevID:    readString(r),
		}

		if parentID := readString(r); parentID != "" {
			if parent := parents[docAndRev{entry.DocID, parentID}]; parent != nil {
				// Clear out the parent rev that was overwritten by this one
				parent.DocID = ""
				parent.RevID = ""
			}
		}
		parents[docAndRev{entry.DocID, entry.RevID}] = entry

		ch.Entries = append(ch.Entries, entry)
	}
	return &ch
}

// Removes the oldest entries to limit the log's length to `maxLength`.
// This is the same as ChangeLog.Truncate except it works directly on the encoded form, which is
// much faster than decoding+truncating+encoding.
func TruncateEncodedChangeLog(r *bytes.Reader, maxLength int, w io.Writer) int {
	since := readSequence(r)
	// Find the starting position of each entry:
	entryPos := make([]int64, 0, 1000)
	for {
		pos, _ := r.Seek(0, 1)
		flags, err := r.ReadByte()
		if err != nil {
			break // eof
		}
		entryPos = append(entryPos, pos)
		readSequence(r)
		skipString(r)
		skipString(r)
		skipString(r)
		if flags > 7 {
			panic(fmt.Sprintf("bad flags %x, entry %d, offset %d", flags, len(entryPos)-1, pos))
		}
	}

	// How many entries to remove?
	remove := len(entryPos) - maxLength
	if remove <= 0 {
		return 0
	}
	// Update the log's Since to the sequence of the last entry being removed:
	r.Seek(entryPos[remove-1]+1, 0)
	since = readSequence(r)

	// Write the updated Since and the remaining entries:
	writeSequence(since, w)
	r.Seek(entryPos[remove], 0)
	io.Copy(w, r)
	return remove
}

//////// UTILITY FUNCTIONS:

func writeSequence(seq uint64, w io.Writer) {
	var buf [16]byte
	len := binary.PutUvarint(buf[0:16], seq)
	w.Write(buf[0:len])
}

func readSequence(r io.ByteReader) uint64 {
	seq, _ := binary.ReadUvarint(r)
	return seq
}

func writeString(s string, w io.Writer) {
	b := []byte(s)
	length := len(b)
	if length > 255 {
		panic("Doc/rev ID too long to encode: " + s)
	}
	if err := binary.Write(w, binary.BigEndian, uint8(length)); err != nil {
		panic("Write failed")
	}
	if _, err := w.Write(b); err != nil {
		panic("writeString failed")
	}
}

func readString(r io.Reader) string {
	var length uint8
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		panic("readString length failed")
	}
	var data [256]byte
	if _, err := io.ReadFull(r, data[0:length]); err != nil {
		panic("readString bytes failed")
	}
	return string(data[0:length])
}

func skipString(r io.ReadSeeker) {
	var length uint8
	binary.Read(r, binary.BigEndian, &length)
	r.Seek(int64(length), 1)
}
