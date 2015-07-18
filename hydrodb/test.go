package main

// log-timestamp 	324245		-- current timestamp
// log.timestamp.0000324244	-- timestamps in time order
// log.timestamp.			-- end of timestamps

import (
	"bytes"
	"io"
	"log"

	"github.com/cznic/kv"
)

func main() {
	db, err := kv.CreateMem(&kv.Options{
		Compare: compare,
	})
	if err != nil {
		log.Fatal(err)
	}
	err = db.Set([]byte("hello.foo.00000"), []byte("whee"))
	if err != nil {
		log.Fatal(err)
	}
	err = db.Set([]byte("hello.foo.00001"), []byte("whee"))
	if err != nil {
		log.Fatal(err)
	}
	err = db.Set([]byte("hello.foo."), []byte("whee"))
	if err != nil {
		log.Fatal(err)
	}
	enum, _, err := db.Seek([]byte("hello.foo."))
	if err != nil {
		log.Fatal(err)
	}
	for {
		k, v, err := enum.Prev()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%s=%s", k, v)
	}
}

var sep = []byte(".")

func compare(k0, k1 []byte) int {
	// TODO avoid the allocations incurred by Split.
	parts0, parts1 := bytes.Split(k0, sep), bytes.Split(k1, sep)
	for i, p0 := range parts0 {
		if i >= len(parts1) {
			return 1
		}
		p1 := parts1[i]
		if r := comparePart(p0, p1); r != 0 {
			return r
		}
	}
	if len(parts0) == len(parts1) {
		return 0
	}
	return -1
}

func comparePart(p0, p1 []byte) int {
	if len(p0) == 0 || len(p1) == 0 {
		// An empty part compares greater than
		// any non-empty part, enabling us to
		// place an easily findable end marker
		// after a section.
		if len(p0) == len(p1) {
			return 0
		}
		if len(p0) == 0 {
			return 1
		}
		return -1
	}
	return bytes.Compare(p0, p1)
}
