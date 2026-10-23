package worker

import "testing"

func TestMediaOutputKeysAreDeterministic(t *testing.T) {
	t.Parallel()
	play, cover := mediaOutputKeys(7, 42)
	if play != "videos/7/processed/42/playback.mp4" {
		t.Fatalf("unexpected playback key %q", play)
	}
	if cover != "covers/7/generated/42/cover.jpg" {
		t.Fatalf("unexpected cover key %q", cover)
	}
}
