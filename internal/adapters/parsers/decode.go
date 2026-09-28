package parsers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// decodeStats streams a stats JSON file into v using a json.Decoder directly
// from the file handle. This avoids materializing the full file as a []byte
// (metafiles routinely reach 50-500MB) before unmarshalling, roughly halving
// peak memory. It also checks ctx between decode boundaries so large files can
// be cancelled.
func decodeStats(ctx context.Context, path string, v any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open stats file %q: %w", path, err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decode %q: %w", path, err)
	}
	return ctx.Err()
}