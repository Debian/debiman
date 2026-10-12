package main

import (
	"io"
	"maps"
	"slices"
	"sync/atomic"

	pb "github.com/Debian/debiman/internal/proto"
	"github.com/Debian/debiman/internal/write"
	"google.golang.org/protobuf/proto"
)

// writeIndex serializes an index for the redirect package (used in
// debiman-auxserver) to dest.
func writeIndex(dest string, gv globalView) error {
	langs := make(map[string]bool)
	sections := make(map[string]bool)
	entries := make([]*pb.IndexEntry, 0, len(gv.xref))
	for _, x := range gv.xref {
		for _, m := range x {
			entries = append(entries, pb.IndexEntry_builder{
				Name:      new(m.Name),
				Suite:     new(m.Package.Suite),
				Binarypkg: new(m.Package.Binarypkg),
				Section:   new(m.Section),
				Language:  new(m.Language),
			}.Build())
			langs[m.Language] = true
			sections[m.Section] = true
			sections[m.MainSection()] = true
		}
	}

	idxb, err := proto.Marshal(pb.Index_builder{
		Entry:    entries,
		Language: slices.Collect(maps.Keys(langs)),
		Section:  slices.Collect(maps.Keys(sections)),
		Suite:    gv.idxSuites,
	}.Build())
	if err != nil {
		return err
	}

	return write.Atomically(dest, false, func(w io.Writer) error {
		_, err := w.Write(idxb)
		if err != nil {
			return err
		}
		atomic.AddUint64(&gv.stats.IndexBytes, uint64(len(idxb)))
		return nil
	})
}
