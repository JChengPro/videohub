package media

import "testing"

func TestProgressPercent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		outTimeMicros  int64
		durationMillis int64
		want           int
	}{
		{name: "start", outTimeMicros: 0, durationMillis: 10_000, want: 0},
		{name: "half", outTimeMicros: 5_000_000, durationMillis: 10_000, want: 50},
		{name: "near end", outTimeMicros: 9_900_000, durationMillis: 10_000, want: 99},
		{name: "bounded before completion", outTimeMicros: 12_000_000, durationMillis: 10_000, want: 99},
		{name: "missing duration", outTimeMicros: 1_000_000, durationMillis: 0, want: 0},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := ProgressPercent(test.outTimeMicros, test.durationMillis); got != test.want {
				t.Fatalf("ProgressPercent() = %d, want %d", got, test.want)
			}
		})
	}
}
