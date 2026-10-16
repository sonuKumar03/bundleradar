package core

import (
	"fmt"
	"slices"
)

// CheckInvariants verifies internal consistency of a parsed bundle and
// returns human-readable warnings for anything that cannot be right. A
// violation means a parser or the stats file produced numbers we should not
// present with confidence.
func (b *Bundle) CheckInvariants() []string {
	var warnings []string

	names := make([]string, 0, len(b.Entrypoints))
	for name := range b.Entrypoints {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		for _, id := range b.Entrypoints[name].ChunkIDs {
			if _, ok := b.FindChunk(id); !ok {
				warnings = append(warnings, fmt.Sprintf("entrypoint %q references unknown chunk %q", name, id))
			}
		}
	}

	js := b.TotalJSBytes()
	if js > 0 && len(b.Entrypoints) > 0 && b.TotalInitialBytes() == 0 {
		warnings = append(warnings, "no initial JavaScript was identified although the bundle contains JavaScript; initial size cannot be trusted (check the stats file and --dist)")
	}

	if len(b.Entrypoints) == 1 {
		var initialChunks int64
		for _, c := range b.Chunks {
			if c.Type == LoadTypeInitial && !c.IsCSS() {
				initialChunks += c.SizeBytes
			}
		}
		if got := b.TotalInitialBytes(); got != initialChunks {
			warnings = append(warnings, fmt.Sprintf("initial size %d bytes does not match the %d bytes of chunks classified as initial", got, initialChunks))
		}
		if lazy := b.TotalAsyncBytes(); b.TotalInitialBytes()+lazy != js {
			warnings = append(warnings, fmt.Sprintf("initial (%d) + lazy (%d) bytes do not add up to the %d bytes of emitted JavaScript", b.TotalInitialBytes(), lazy, js))
		}
	}

	perChunk := make(map[string]int64)
	for _, m := range b.Modules {
		for cid, n := range m.ChunkBytes {
			perChunk[cid] += n
		}
	}
	for _, c := range b.Chunks {
		if perChunk[c.ID] > c.SizeBytes {
			warnings = append(warnings, fmt.Sprintf("modules in chunk %q add up to %d bytes, more than the chunk's %d bytes", c.ID, perChunk[c.ID], c.SizeBytes))
		}
	}
	return warnings
}
