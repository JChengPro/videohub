package worker

import "testing"

func TestMediaOutputKeysAreDeterministic(t *testing.T) {
	t.Parallel()
	play, covers := mediaOutputKeys(7, 42)
	if play != "videos/7/processed/42/playback.mp4" {
		t.Fatalf("unexpected playback key %q", play)
	}
	want := []string{
		"covers/7/generated/42/cover_1.jpg",
		"covers/7/generated/42/cover_2.jpg",
		"covers/7/generated/42/cover_3.jpg",
	}
	if len(covers) != len(want) {
		t.Fatalf("got %d covers, want %d", len(covers), len(want))
	}
	for index := range want {
		if covers[index] != want[index] {
			t.Fatalf("cover[%d] = %q, want %q", index, covers[index], want[index])
		}
	}
}
