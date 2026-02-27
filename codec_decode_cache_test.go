package rap

import (
	"reflect"
	"testing"

	"github.com/woozymasta/rvcfg"
)

func TestDecodeClassBodyAtWithEnd_CacheHitKeepsEndOffset(t *testing.T) {
	t.Parallel()

	input := rvcfg.File{
		Statements: []rvcfg.Statement{
			{
				Kind: rvcfg.NodeProperty,
				Property: &rvcfg.PropertyAssign{
					Name: "x",
					Value: rvcfg.Value{
						Kind: rvcfg.ValueScalar,
						Raw:  "1",
					},
				},
			},
		},
	}

	data, err := EncodeAST(input, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeAST() error: %v", err)
	}

	ctx := &decodeContext{
		reader:   newBinaryReader(data),
		bodyMemo: make(map[int]decodedClassBody),
		bodyBusy: make(map[int]struct{}),
	}

	firstBody, firstEnd, err := ctx.decodeClassBodyAtWithEnd(16)
	if err != nil {
		t.Fatalf("decodeClassBodyAtWithEnd(first) error: %v", err)
	}

	if firstEnd <= 16 {
		t.Fatalf("expected first end offset > 16, got=%d", firstEnd)
	}

	secondBody, secondEnd, err := ctx.decodeClassBodyAtWithEnd(16)
	if err != nil {
		t.Fatalf("decodeClassBodyAtWithEnd(second) error: %v", err)
	}

	if secondEnd != firstEnd {
		t.Fatalf("cache-hit end offset mismatch: first=%d second=%d", firstEnd, secondEnd)
	}

	if !reflect.DeepEqual(firstBody, secondBody) {
		t.Fatalf("cache-hit body mismatch:\nfirst=%#v\nsecond=%#v", firstBody, secondBody)
	}
}
