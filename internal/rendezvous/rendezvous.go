package rendezvous

import (
	"encoding/binary"
	"hash/fnv"
	"sync"
)

var defaultHasher = newHasher()

type hasher struct {
	mu             sync.RWMutex
	candidateCache map[string]uint64
}

func newHasher() *hasher {
	return &hasher{
		candidateCache: make(map[string]uint64),
	}
}

func HighestScore(candidates []string, key string) (uint64, string) {
	return defaultHasher.highestScore(candidates, key)
}

func (h *hasher) highestScore(candidates []string, key string) (uint64, string) {
	if len(candidates) == 0 {
		return 0, ""
	}

	keyHash := hashString(key)
	var (
		bestScore     uint64
		bestCandidate string
	)

	for _, candidate := range candidates {
		score := h.scoreCandidate(keyHash, candidate)
		if bestCandidate == "" || score > bestScore {
			bestScore = score
			bestCandidate = candidate
		}
	}

	return bestScore, bestCandidate
}

func (h *hasher) scoreCandidate(keyHash uint64, candidate string) uint64 {
	candidateHash := h.candidateHash(candidate)
	return hashPair(keyHash, candidateHash)
}

func (h *hasher) candidateHash(candidate string) uint64 {
	h.mu.RLock()
	cached, ok := h.candidateCache[candidate]
	h.mu.RUnlock()
	if ok {
		return cached
	}

	computed := hashString(candidate)

	h.mu.Lock()
	if cached, ok = h.candidateCache[candidate]; ok {
		h.mu.Unlock()
		return cached
	}
	h.candidateCache[candidate] = computed
	h.mu.Unlock()

	return computed
}

func hashString(value string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(value))
	return h.Sum64()
}

func hashPair(a uint64, b uint64) uint64 {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint64(buf[:8], a)
	binary.LittleEndian.PutUint64(buf[8:], b)
	h := fnv.New64a()
	_, _ = h.Write(buf)
	return h.Sum64()
}
