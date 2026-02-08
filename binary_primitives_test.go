package rap

import (
	"strconv"
	"testing"
)

func TestCompressedIntRoundTrip(t *testing.T) {
	t.Parallel()

	values := []int{
		0,
		1,
		3,
		7,
		63,
		127,
		128,
		130,
		255,
		300,
		4096,
		16384,
		21100,
	}

	for _, value := range values {
		value := value

		t.Run("v_"+strconv.Itoa(value), func(t *testing.T) {
			t.Parallel()

			w := newBinaryWriterWithCapacity(16)
			if err := w.writeCompressedInt(value); err != nil {
				t.Fatalf("writeCompressedInt(%d) error: %v", value, err)
			}

			r := newBinaryReader(w.bytes())
			got, err := r.readCompressedInt()
			if err != nil {
				t.Fatalf("readCompressedInt() error: %v", err)
			}

			if got != value {
				t.Fatalf("compressed int roundtrip mismatch: got=%d want=%d bytes=%v", got, value, w.bytes())
			}
		})
	}
}
