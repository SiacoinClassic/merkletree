// +build gofuzz

package merkletree

import (
	"bytes"
	"crypto/sha256"
	"io"
)

// Fuzz is called by go-fuzz to look for inputs to BuildReaderProof that will
// not verify correctly.
func Fuzz(data []byte) int {
	// Use the first two bytes to determine the proof index.
	if len(data) < 2 {
		return -1
	}
	index := 256*uint64(data[0]) + uint64(data[1])
	data = data[2:]

	// Build a reader proof for index 'index' using the remaining data as input
	// to the reader. '64' is chosen as the only input size because that is the
	// size relevant to the Sia project.
	merkleRoot, proofSet, numLeaves, err := BuildReaderProof(bytes.NewReader(data), sha256.New(), 64, index)
	if err != nil {
		return 0
	}
	if !VerifyProof(sha256.New(), merkleRoot, proofSet, index, numLeaves) {
		panic("verification failed!")
	}

	// Output is more interesting when there is enough data to contain the
	// index.
	if uint64(len(data)) > 64*index {
		return 1
	}
	return 0
}

// FuzzReadSubTreesWithProof can be used by go-fuzz to test creating a merkle
// tree from cached subTrees and creating/proving a merkle proof on this tree.
func FuzzReadSubTreesWithProof(data []byte) int {
	// Use the first two bytes to determine the proof index.
	if len(data) < 2 {
		return -1
	}
	// Use the first two bytes to determine the proof index.
	index := 256*uint64(data[0]) + uint64(data[1])
	tree := New(sha256.New())
	tree.SetIndex(index)
	data = data[2:]

	subTreeSize := 1 + 32 // 1 byte height + 32 bytes hash
	err := tree.readSubTrees(bytes.NewReader(data))
	if err != nil && err != io.ErrUnexpectedEOF {
		return 0
	} else if err == io.ErrUnexpectedEOF && len(data) < subTreeSize {
		return -1
	}

	// Create and verify the proof.
	merkleRoot, proofSet, proofIndex, numLeaves := tree.Prove()
	if len(proofSet) == 0 {
		// proofIndex wasn't reached while creating proof.
		return 0
	}
	if !VerifyProof(sha256.New(), merkleRoot, proofSet, proofIndex, numLeaves) {
		panic("verification failed!")
	}
	// Output is more interesting when there is enough data to contain the
	// index.
	if uint64(len(data)) > uint64(subTreeSize)*index {
		return 1
	}
	return 0
}

// FuzzReadSubTreesNoProof can be used by go-fuzz to test creating a merkle
// tree from cached subTrees.
func FuzzReadSubTreesNoProof(data []byte) int {
	tree := New(sha256.New())

	subTreeSize := 1 + 32 // 1 byte height + 32 bytes hash
	err := tree.readSubTrees(bytes.NewReader(data))
	if err != nil && err != io.ErrUnexpectedEOF {
		return 0
	} else if err == io.ErrUnexpectedEOF && len(data) < subTreeSize {
		return -1
	}
	if tree.head != nil && tree.Root() == nil {
		panic("root shouldn't be nil for a non-empty tree")
	}
	// The data should at least contain 2 subtrees.
	if len(data) > 2*subTreeSize {
		return 1
	}
	return 0
}

// readSubTrees is a helper function that maps the data from a io.Reader to
// subTrees and adds them to a tree.
func (t *Tree) readSubTrees(r io.Reader) error {
	// 32 is the length of a sha256 hash and 1 byte is used for the height of
	// the subTree.
	subTreeSize := 1 + 32
	for {
		subTree := make([]byte, subTreeSize)
		_, readErr := io.ReadFull(r, subTree)
		if readErr == io.EOF {
			// All data has been read.
			break
		} else if readErr != nil {
			return readErr
		}
		// The first byte of the subTree are mapped to a height in range
		// [0,4].
		height := int(subTree[0]) % 5
		sum := subTree[4:]
		if height > 0 {
			if err := t.PushSubTree(height, sum); err != nil {
				return err
			}
		} else {
			t.Push(sum)
		}
	}
	return nil
}
