package pfpgp

import (
	"crypto/sha256"
)

// The index is SHA256 hashed, thus the index is 16 bytes wide.
const IndexedKeySetHashSize = sha256.Size

// IndexedKeySet is used to sort PGP keys and add them in a
// unique way to avoid keys to be listed multiple times.
//
// We SHA256 each key to only allow a key to be added once.
type IndexedKeySet struct {
	keys map[[IndexedKeySetHashSize]byte][]byte
}

// NewIndexedKeySet creates a new IndexedKeySet.
func NewIndexedKeySet() *IndexedKeySet {
	return &IndexedKeySet{make(map[[IndexedKeySetHashSize]byte][]byte)}
}

// Add can be used to add a key to the IndexedKeySet.
//
// The 'key' provided is a full ASCII formatted PGP PUBLIC KEY block.
//
// A SHA256 is used to hash the key to avoid duplicates of the exact same key.
func (keyset *IndexedKeySet) Add(key string) {
	bkey := []byte(key)

	keyset.keys[sha256.Sum256(bkey)] = bkey
}

// ToBytes converts the keyset into ASCII output.
//
// Unix-style newlines (LF-only) are added in between
// the keys to keep them separate in the output.
func (keyset *IndexedKeySet) ToBytes() (output []byte) {
	// Add each key in the set
	for k := range keyset.keys {
		// Add the key
		output = append(output, keyset.keys[k][:]...)

		// Add a \n (LF)
		output = append(output, byte(0x0a))
	}

	return
}
