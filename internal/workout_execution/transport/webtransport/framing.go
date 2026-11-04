package webtransport

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// maxMessageBytes bounds a single protobuf message. A 17-keypoint frame is ~400 bytes; 64 KiB leaves
// generous headroom while preventing a hostile client from forcing large allocations.
const maxMessageBytes = 64 * 1024

var errMessageTooLarge = errors.New("webtransport: message exceeds size limit")

// readMessage reads one uvarint-length-prefixed message from r.
func readMessage(r *bufio.Reader) ([]byte, error) {
	n, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	if n > maxMessageBytes {
		return nil, fmt.Errorf("%w: %d bytes", errMessageTooLarge, n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return buf, nil
}

// writeMessage writes one uvarint-length-prefixed message in a single Write call so frames never interleave.
func writeMessage(w io.Writer, payload []byte) error {
	if len(payload) > maxMessageBytes {
		return errMessageTooLarge
	}
	buf := make([]byte, binary.MaxVarintLen64+len(payload))
	n := binary.PutUvarint(buf, uint64(len(payload)))
	copy(buf[n:], payload)
	_, err := w.Write(buf[:n+len(payload)])
	return err
}
