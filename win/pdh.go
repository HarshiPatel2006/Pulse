//go:build windows

package win

import (
	"unsafe"
)

var (
	pPdhOpenQuery                = pdh.NewProc("PdhOpenQueryW")
	pPdhAddEnglishCounter        = pdh.NewProc("PdhAddEnglishCounterW")
	pPdhCollectQueryData         = pdh.NewProc("PdhCollectQueryData")
	pPdhGetFormattedCounterArray = pdh.NewProc("PdhGetFormattedCounterArrayW")
	pPdhGetFormattedCounterValue = pdh.NewProc("PdhGetFormattedCounterValue")
	pPdhCloseQuery               = pdh.NewProc("PdhCloseQuery")
)

const (
	pdhFmtDouble   = 0x00000200
	pdhFmtNoCap100 = 0x00008000
	pdhMoreData    = 0x800007D2
)

// Query is a set of performance counters collected together.
type Query struct {
	h        uintptr
	counters map[string]uintptr
	buf      []byte
}

func NewQuery() *Query {
	q := &Query{counters: map[string]uintptr{}}
	if call(pPdhOpenQuery, 0, 0, uintptr(unsafe.Pointer(&q.h))) != 0 {
		return nil
	}
	return q
}

// Add registers a counter path (English names, wildcards allowed). Returns false if unavailable.
func (q *Query) Add(key, path string) bool {
	if q == nil {
		return false
	}
	var c uintptr
	if call(pPdhAddEnglishCounter, q.h, Str(path), 0, uintptr(unsafe.Pointer(&c))) != 0 {
		return false
	}
	q.counters[key] = c
	return true
}

func (q *Query) Has(key string) bool { return q != nil && q.counters[key] != 0 }

func (q *Query) Collect() {
	if q != nil {
		call(pPdhCollectQueryData, q.h)
	}
}

func (q *Query) Value(key string) (float64, bool) {
	c := q.counterOf(key)
	if c == 0 {
		return 0, false
	}
	var v struct {
		Status uint32
		_      uint32
		Val    float64
	}
	if call(pPdhGetFormattedCounterValue, c, pdhFmtDouble|pdhFmtNoCap100, 0, uintptr(unsafe.Pointer(&v))) != 0 {
		return 0, false
	}
	return v.Val, v.Status <= 1
}

func (q *Query) counterOf(key string) uintptr {
	if q == nil {
		return 0
	}
	return q.counters[key]
}

type pdhItem struct {
	Name   *uint16
	Status uint32
	_      uint32
	Val    float64
}

// Array calls fn(instance, value) for each instance of a wildcard counter.
func (q *Query) Array(key string, fn func(name string, v float64)) {
	c := q.counterOf(key)
	if c == 0 {
		return
	}
	var size, count uint32
	r := call(pPdhGetFormattedCounterArray, c, pdhFmtDouble|pdhFmtNoCap100, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), 0)
	if uint32(r) != pdhMoreData || size == 0 {
		return
	}
	if cap(q.buf) < int(size) {
		q.buf = make([]byte, size)
	}
	b := q.buf[:size]
	if call(pPdhGetFormattedCounterArray, c, pdhFmtDouble|pdhFmtNoCap100, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&b[0]))) != 0 {
		return
	}
	items := unsafe.Slice((*pdhItem)(unsafe.Pointer(&b[0])), count)
	for _, it := range items {
		if it.Status <= 1 {
			fn(GoStr(uintptr(unsafe.Pointer(it.Name))), it.Val)
		}
	}
}

func (q *Query) Close() {
	if q != nil {
		call(pPdhCloseQuery, q.h)
	}
}
