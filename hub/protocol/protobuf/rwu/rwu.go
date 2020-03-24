// Utilities for reading and writing of binary data
package rwu

import (
	"encoding/binary"
	"io"
)

func ReadInt32(r io.Reader) (int32, error) {
	var c int32
	err := binary.Read(r, binary.LittleEndian, &c)
	return c, err
}
