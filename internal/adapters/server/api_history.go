package server

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
)

// BuildCheckpoint represents a point-in-time bundle scan checkpoint.
type BuildCheckpoint struct {
	ID               string     `json:"id"`
	Timestamp        time.Time  `json:"timestamp"`
	Label            string     `json:"label"`
	InitialBytes     int64      `json:"initialBytes"`
	InitialGzipBytes int64      `json:"initialGzipBytes"`
	AsyncBytes       int64      `json:"asyncBytes"`
	TotalBytes       int64      `json:"totalBytes"`
	ChunksCount      int        `json:"chunksCount"`
	PkgsCount        int        `json:"pkgsCount"`
	IsBaseline       bool       `json:"isBaseline"`
	Bundle           *BundleDTO `json:"bundle,omitempty"`
}

// PackageDiffDTO describes size movement for a single package between two checkpoints.
type PackageDiffDTO struct {
	Name               string `json:"name"`
	BaseInitialBytes   int64  `json:"baseInitialBytes"`
	TargetInitialBytes int64  `json:"targetInitialBytes"`
	DeltaInitialBytes  int64  `json:"deltaInitialBytes"`
	BaseTotalBytes     int64  `json:"baseTotalBytes"`
	TargetTotalBytes   int64  `json:"targetTotalBytes"`
	DeltaTotalBytes    int64  `json:"deltaTotalBytes"`
	Status             string `json:"status"` // "ELIMINATED", "REDUCED", "REGRESSED", "ADDED", "UNCHANGED"
	IngressPath        string `json:"ingressPath,omitempty"`
}

// DiffDTO describes bundle drift between two checkpoints.
type DiffDTO struct {
	BaseID              string           `json:"baseId"`
	TargetID            string           `json:"targetId"`
	BaseInitialBytes    int64            `json:"baseInitialBytes"`
	TargetInitialBytes  int64            `json:"targetInitialBytes"`
	InitialDeltaBytes   int64            `json:"initialDeltaBytes"`
	InitialDeltaPercent float64          `json:"initialDeltaPercent"`
	TotalDeltaBytes     int64            `json:"totalDeltaBytes"`
	PackageDiffs        []PackageDiffDTO `json:"packageDiffs"`
}

// RecordCheckpoint saves a bundle as an immutable build version checkpoint.
func (s *Server) RecordCheckpoint(bundle *core.Bundle, label string) *BuildCheckpoint {
	dto := s.BundleToDTO(bundle)

	s.mu.Lock()
	defer s.mu.Unlock()

	cpID := fmt.Sprintf("build-%d", len(s.checkpoints)+1)
	if label == "" {
		label = fmt.Sprintf("Build #%d", len(s.checkpoints)+1)
	}

	var initBytes, initGzip, asyncBytes int64
	if len(dto.Entrypoints) > 0 {
		mainEp, ok := dto.Entrypoints["main"]
		if !ok {
			for _, ep := range dto.Entrypoints {
				mainEp = ep
				break
			}
		}
		initBytes = mainEp.InitialBytes
		initGzip = mainEp.InitialGzipBytes
		asyncBytes = mainEp.AsyncBytes
	} else {
		for _, ch := range dto.Chunks {
			if ch.Type == "initial" {
				initBytes += ch.SizeBytes
				initGzip += ch.GzipBytes
			} else {
				asyncBytes += ch.SizeBytes
			}
		}
	}

	isBaseline := len(s.checkpoints) == 0
	if isBaseline {
		s.baselineID = cpID
	}

	cp := &BuildCheckpoint{
		ID:               cpID,
		Timestamp:        time.Now(),
		Label:            label,
		InitialBytes:     initBytes,
		InitialGzipBytes: initGzip,
		AsyncBytes:       asyncBytes,
		TotalBytes:       dto.TotalBytes,
		ChunksCount:      len(dto.Chunks),
		PkgsCount:        len(dto.TopPackages),
		IsBaseline:       isBaseline,
		Bundle:           dto,
	}

	s.checkpoints = append(s.checkpoints, cp)
	s.broadcastBuild(cp)
	return cp
}

// BaselineID returns the ID of the current baseline checkpoint.
func (s *Server) BaselineID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.baselineID
}

// CheckpointCount returns the number of recorded checkpoints.
func (s *Server) CheckpointCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.checkpoints)
}

// GetCheckpoint returns the checkpoint by ID.
func (s *Server) GetCheckpoint(id string) *BuildCheckpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, cp := range s.checkpoints {
		if cp.ID == id {
			return cp
		}
	}
	return nil
}

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	type historyResp struct {
		BaselineID  string             `json:"baselineId"`
		Checkpoints []*BuildCheckpoint `json:"checkpoints"`
	}

	resp := historyResp{
		BaselineID:  s.baselineID,
		Checkpoints: s.checkpoints,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSetBaseline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		id = body.ID
	}

	if id == "" {
		http.Error(w, `{"error":"missing build id"}`, http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	found := false
	for _, cp := range s.checkpoints {
		if cp.ID == id {
			found = true
			cp.IsBaseline = true
		} else {
			cp.IsBaseline = false
		}
	}
	if found {
		s.baselineID = id
	}
	s.mu.Unlock()

	if !found {
		http.Error(w, `{"error":"checkpoint not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "baselineId": id})
}

func (s *Server) handleGetDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	baseID := r.URL.Query().Get("base")
	targetID := r.URL.Query().Get("target")

	s.mu.RLock()
	if baseID == "" {
		baseID = s.baselineID
	}
	if targetID == "" && len(s.checkpoints) > 0 {
		targetID = s.checkpoints[len(s.checkpoints)-1].ID
	}

	var baseCP, targetCP *BuildCheckpoint
	for _, cp := range s.checkpoints {
		if cp.ID == baseID {
			baseCP = cp
		}
		if cp.ID == targetID {
			targetCP = cp
		}
	}
	s.mu.RUnlock()

	if baseCP == nil || targetCP == nil {
		http.Error(w, `{"error":"one or both checkpoints not found"}`, http.StatusNotFound)
		return
	}

	diff := computeDiff(baseCP, targetCP)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(diff)
}

// computeDiff delegates to core/diff.Calculate so the UI shares exact diff
// semantics with the CLI and MCP, then reshapes the result into the wire DTO.
// Status classification is core/diff.Status; here we additionally track
// per-package initial-byte movement, which is a UI-only view over the same data.
func computeDiff(baseCP, targetCP *BuildCheckpoint) *DiffDTO {
	baseBundle := bundleFromCheckpoint(baseCP)
	targetBundle := bundleFromCheckpoint(targetCP)

	result := diff.Calculate(baseBundle, targetBundle, diff.Options{})

	basePkgs := make(map[string]PackageDTO)
	if baseCP.Bundle != nil {
		for _, p := range baseCP.Bundle.TopPackages {
			basePkgs[p.Name] = p
		}
	}
	targetPkgs := make(map[string]PackageDTO)
	if targetCP.Bundle != nil {
		for _, p := range targetCP.Bundle.TopPackages {
			targetPkgs[p.Name] = p
		}
	}

	packageDiffs := make([]PackageDiffDTO, 0, len(result.Packages)+len(result.UnchangedPackages))
	for _, pd := range append(append([]diff.PackageDelta{}, result.Packages...), result.UnchangedPackages...) {
		var baseInit, targetInit int64
		var ingress string
		if bp, ok := basePkgs[pd.Name]; ok {
			baseInit = bp.InitialBytes
			ingress = bp.IngressPath
		}
		if tp, ok := targetPkgs[pd.Name]; ok {
			targetInit = tp.InitialBytes
			if ingress == "" {
				ingress = tp.IngressPath
			}
		}

		// Reclassify on initial bytes, the metric this DTO exposes. Total-byte
		// status from core/diff is retained in result.Packages for other views.
		status := diff.Status(baseInit, targetInit)

		packageDiffs = append(packageDiffs, PackageDiffDTO{
			Name:               pd.Name,
			BaseInitialBytes:   baseInit,
			TargetInitialBytes: targetInit,
			DeltaInitialBytes:  targetInit - baseInit,
			BaseTotalBytes:     pd.BaseBytes,
			TargetTotalBytes:   pd.CurrBytes,
			DeltaTotalBytes:    pd.DeltaBytes,
			Status:             status,
			IngressPath:        ingress,
		})
	}

	// Sort package diffs: largest absolute delta in initial bytes first, then total bytes
	slices.SortFunc(packageDiffs, func(a, b PackageDiffDTO) int {
		absA := math.Abs(float64(a.DeltaInitialBytes))
		absB := math.Abs(float64(b.DeltaInitialBytes))
		if absA != absB {
			return cmp.Compare(absB, absA)
		}
		return strings.Compare(a.Name, b.Name)
	})

	initDelta := targetCP.InitialBytes - baseCP.InitialBytes
	var deltaPct float64
	if baseCP.InitialBytes > 0 {
		deltaPct = (float64(initDelta) / float64(baseCP.InitialBytes)) * 100
	}

	return &DiffDTO{
		BaseID:              baseCP.ID,
		TargetID:            targetCP.ID,
		BaseInitialBytes:    baseCP.InitialBytes,
		TargetInitialBytes:  targetCP.InitialBytes,
		InitialDeltaBytes:   initDelta,
		InitialDeltaPercent: math.Round(deltaPct*100) / 100,
		TotalDeltaBytes:     targetCP.TotalBytes - baseCP.TotalBytes,
		PackageDiffs:        packageDiffs,
	}
}

// bundleFromCheckpoint reconstructs a minimal core.Bundle from a checkpoint's
// stored DTO so checkpoint diffs reuse core/diff.Calculate. Reconstructs only
// modules and chunks, which is all Calculate consumes for package deltas.
func bundleFromCheckpoint(cp *BuildCheckpoint) *core.Bundle {
	if cp == nil || cp.Bundle == nil {
		return nil
	}
	b := core.NewBundle(core.Metadata{})
	chunkType := make(map[string]core.LoadType)
	for _, ch := range cp.Bundle.Chunks {
		id := ch.Name
		lt := core.LoadTypeInitial
		if ch.Type == "async" {
			lt = core.LoadTypeAsync
		}
		b.AddChunk(core.Chunk{ID: id, Name: ch.Name, SizeBytes: ch.SizeBytes, GzipBytes: ch.GzipBytes, Type: lt})
		chunkType[ch.Name] = lt
	}
	for _, p := range cp.Bundle.TopPackages {
		mod := core.Module{
			ID:        "pkg:" + p.Name,
			Package:   p.Name,
			SizeBytes: p.SizeBytes,
			GzipBytes: p.GzipBytes,
		}
		if p.InitialBytes > 0 {
			mod.ChunkIDs = []string{"__initial__"}
		} else if p.AsyncBytes > 0 {
			mod.ChunkIDs = []string{"__async__"}
		}
		b.AddModule(mod)
	}
	return b
}

func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 16)
	s.registerClient(ch)
	defer s.unregisterClient(ch)

	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	done := r.Context().Done()
	for {
		select {
		case <-done:
			return
		case msg := <-ch:
			fmt.Fprintf(w, "event: build\ndata: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (s *Server) registerClient(ch chan []byte) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	s.clients[ch] = struct{}{}
}

func (s *Server) unregisterClient(ch chan []byte) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	delete(s.clients, ch)
	close(ch)
}

func (s *Server) broadcastBuild(cp *BuildCheckpoint) {
	data, err := json.Marshal(cp)
	if err != nil {
		return
	}
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- data:
		default:
		}
	}
}
