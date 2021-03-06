package channels

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/couchbaselabs/go.assert"
)

func e(seq uint64, docid string, revid string) *LogEntry {
	return &LogEntry{
		Sequence: seq,
		DocID:    docid,
		RevID:    revid,
	}
}

func mklog(since uint64, entries ...*LogEntry) ChangeLog {
	return ChangeLog{Since: since, Entries: entries}
}

func TestEmptyLog(t *testing.T) {
	var cl ChangeLog
	assert.Equals(t, len(cl.EntriesAfter(1234)), 0)

	cl.Add(*e(1, "foo", "1-a"))
	assert.Equals(t, cl.Since, uint64(0))
	assert.DeepEquals(t, cl.EntriesAfter(0), []*LogEntry{e(1, "foo", "1-a")})
	assert.DeepEquals(t, cl.EntriesAfter(1), []*LogEntry{})
}

func TestAddInOrder(t *testing.T) {
	var cl ChangeLog
	cl.Add(*e(1, "foo", "1-a"))
	cl.Add(*e(2, "bar", "1-a"))
	assert.DeepEquals(t, cl.EntriesAfter(0), []*LogEntry{e(1, "foo", "1-a"), e(2, "bar", "1-a")})
	assert.DeepEquals(t, cl.EntriesAfter(1), []*LogEntry{e(2, "bar", "1-a")})
	assert.DeepEquals(t, cl.EntriesAfter(2), []*LogEntry{})
	cl.Add(*e(3, "zog", "1-a"))
	assert.DeepEquals(t, cl.EntriesAfter(2), []*LogEntry{e(3, "zog", "1-a")})
	assert.DeepEquals(t, cl, mklog(0, e(1, "foo", "1-a"), e(2, "bar", "1-a"), e(3, "zog", "1-a")))
}

func TestAddOutOfOrder(t *testing.T) {
	var cl ChangeLog
	cl.Add(*e(20, "bar", "1-a"))
	cl.Add(*e(10, "foo", "1-a"))
	assert.Equals(t, cl.Since, uint64(9))
	assert.DeepEquals(t, cl.EntriesAfter(0), []*LogEntry{e(20, "bar", "1-a"), e(10, "foo", "1-a")})
	assert.DeepEquals(t, cl.EntriesAfter(20), []*LogEntry{e(10, "foo", "1-a")})
	assert.DeepEquals(t, cl.EntriesAfter(10), []*LogEntry{})
	cl.Add(*e(30, "zog", "1-a"))
	assert.DeepEquals(t, cl.EntriesAfter(20), []*LogEntry{e(10, "foo", "1-a"), e(30, "zog", "1-a")})
	assert.DeepEquals(t, cl.EntriesAfter(10), []*LogEntry{e(30, "zog", "1-a")})
	assert.DeepEquals(t, cl, mklog(9, e(20, "bar", "1-a"), e(10, "foo", "1-a"), e(30, "zog", "1-a")))
}

func TestReplace(t *testing.T) {
	// Add three sequences in order:
	var cl ChangeLog
	cl.Add(*e(1, "foo", "1-a"))
	cl.Add(*e(2, "bar", "1-a"))
	cl.Add(*e(3, "zog", "1-a"))

	// Replace 'foo'
	cl.Update(*e(4, "foo", "2-b"), "1-a")
	assert.DeepEquals(t, cl, mklog(0, e(1, "", ""), e(2, "bar", "1-a"), e(3, "zog", "1-a"), e(4, "foo", "2-b")))

	// Replace 'zog'
	cl.Update(*e(5, "zog", "2-b"), "1-a")
	assert.DeepEquals(t, cl, mklog(0, e(1, "", ""), e(2, "bar", "1-a"), e(3, "", ""), e(4, "foo", "2-b"), e(5, "zog", "2-b")))

	// Replace 'zog' again
	cl.Update(*e(6, "zog", "3-c"), "2-b")
	assert.DeepEquals(t, cl, mklog(0, e(1, "", ""), e(2, "bar", "1-a"), e(3, "", ""), e(4, "foo", "2-b"), e(5, "", ""), e(6, "zog", "3-c")))
}

func TestTruncate(t *testing.T) {
	const maxLogLength = 50
	var cl ChangeLog
	for i := 1; i <= 2*maxLogLength; i++ {
		cl.Add(*e(uint64(i), "foo", fmt.Sprintf("%d-x", i)))
		cl.TruncateTo(maxLogLength)
	}
	assert.Equals(t, len(cl.Entries), maxLogLength)
	assert.Equals(t, int(cl.Since), maxLogLength)
}

func TestChangeLogEncoding(t *testing.T) {
	assert.Equals(t, Deleted, 1)
	assert.Equals(t, Removed, 2)
	assert.Equals(t, Hidden, 4)

	var cl ChangeLog
	cl.Add(*e(20, "some document", "1-ajkljkjklj"))
	cl.Add(*e(666, "OtherDocument", "666-fjkldfjdkfjd"))
	cl.Add(*e(123456, "a", "5-cafebabe"))

	var w bytes.Buffer
	cl.Encode(&w)
	data := w.Bytes()
	assert.DeepEquals(t, data, []byte{0x13, 0x0, 0x14, 0xd, 0x73, 0x6f, 0x6d, 0x65, 0x20, 0x64, 0x6f, 0x63, 0x75, 0x6d, 0x65, 0x6e, 0x74, 0xc, 0x31, 0x2d, 0x61, 0x6a, 0x6b, 0x6c, 0x6a, 0x6b, 0x6a, 0x6b, 0x6c, 0x6a, 0x0, 0x0, 0x9a, 0x5, 0xd, 0x4f, 0x74, 0x68, 0x65, 0x72, 0x44, 0x6f, 0x63, 0x75, 0x6d, 0x65, 0x6e, 0x74, 0x10, 0x36, 0x36, 0x36, 0x2d, 0x66, 0x6a, 0x6b, 0x6c, 0x64, 0x66, 0x6a, 0x64, 0x6b, 0x66, 0x6a, 0x64, 0x0, 0x0, 0xc0, 0xc4, 0x7, 0x1, 0x61, 0xa, 0x35, 0x2d, 0x63, 0x61, 0x66, 0x65, 0x62, 0x61, 0x62, 0x65, 0x0})

	cl2 := DecodeChangeLog(bytes.NewReader(data))
	assert.Equals(t, cl2.Since, cl.Since)
	assert.Equals(t, len(cl2.Entries), len(cl.Entries))
	for i, entry := range cl2.Entries {
		assert.DeepEquals(t, entry, cl.Entries[i])
	}

	// Append a new entry to the encoded bytes:
	newEntry := LogEntry{
		Sequence: 99,
		DocID:    "some document",
		RevID:    "22-x",
		Flags:    Removed,
	}
	var wNew bytes.Buffer
	newEntry.Encode(&wNew, "1-ajkljkjklj") // It will replace the first entry
	moreData := append(data, wNew.Bytes()...)

	cl2 = DecodeChangeLog(bytes.NewReader(moreData))
	assert.Equals(t, cl2.Since, cl.Since)
	assert.Equals(t, len(cl2.Entries), len(cl.Entries)+1)
	assert.DeepEquals(t, cl2.Entries[len(cl2.Entries)-1], &newEntry)
	assert.Equals(t, cl2.Entries[0].DocID, "") // was replaced by newEntry
	assert.Equals(t, cl2.Entries[0].RevID, "")

	// Truncate cl2's encoded data:
	var wTrunc bytes.Buffer
	removed := TruncateEncodedChangeLog(bytes.NewReader(moreData), 2, &wTrunc)
	assert.Equals(t, removed, 2)
	data3 := wTrunc.Bytes()
	assert.DeepEquals(t, data3, []byte{0x9a, 0x5, 0x0, 0xc0, 0xc4, 0x7, 0x1, 0x61, 0xa, 0x35, 0x2d, 0x63, 0x61, 0x66, 0x65, 0x62, 0x61, 0x62, 0x65, 0x0, 0x2, 0x63, 0xd, 0x73, 0x6f, 0x6d, 0x65, 0x20, 0x64, 0x6f, 0x63, 0x75, 0x6d, 0x65, 0x6e, 0x74, 0x4, 0x32, 0x32, 0x2d, 0x78, 0xc, 0x31, 0x2d, 0x61, 0x6a, 0x6b, 0x6c, 0x6a, 0x6b, 0x6a, 0x6b, 0x6c, 0x6a})

	cl3 := DecodeChangeLog(bytes.NewReader(data3))
	assert.Equals(t, cl3.Since, uint64(666))
	assert.Equals(t, len(cl3.Entries), 2)
	assert.DeepEquals(t, cl3.Entries[0], cl2.Entries[2])
	assert.DeepEquals(t, cl3.Entries[1], cl2.Entries[3])
}
