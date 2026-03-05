package rendezvous

import "testing"

func TestHighestScore_EmptyCandidates(t *testing.T) {
	t.Parallel()

	score, candidate := HighestScore(nil, "key-1")
	if score != 0 {
		t.Fatalf("score mismatch: got %d want %d", score, 0)
	}
	if candidate != "" {
		t.Fatalf("candidate mismatch: got %q want empty", candidate)
	}
}

func TestHighestScore_Deterministic(t *testing.T) {
	t.Parallel()

	candidates := []string{"a", "b", "c", "d"}
	score1, candidate1 := HighestScore(candidates, "tenant-42")
	score2, candidate2 := HighestScore(candidates, "tenant-42")

	if score1 != score2 {
		t.Fatalf("score mismatch: got %d want %d", score2, score1)
	}
	if candidate1 != candidate2 {
		t.Fatalf("candidate mismatch: got %q want %q", candidate2, candidate1)
	}
}

func TestHighestScore_PicksBestCandidate(t *testing.T) {
	t.Parallel()

	candidates := []string{"svc-a", "svc-b", "svc-c"}
	score, winner := HighestScore(candidates, "request-key")

	var (
		expectedScore uint64
		expectedName  string
	)
	for _, candidate := range candidates {
		current := defaultHasher.scoreCandidate(hashString("request-key"), candidate)
		if expectedName == "" || current > expectedScore {
			expectedScore = current
			expectedName = candidate
		}
	}

	if score != expectedScore {
		t.Fatalf("score mismatch: got %d want %d", score, expectedScore)
	}
	if winner != expectedName {
		t.Fatalf("winner mismatch: got %q want %q", winner, expectedName)
	}
}
