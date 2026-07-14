package main

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// strictlyIncreasing reports whether every element is strictly greater than the
// one before it.
func strictlyIncreasing(t *testing.T, xs []float64) {
	t.Helper()
	for i := 1; i < len(xs); i++ {
		require.Greater(t, xs[i], xs[i-1],
			"expected strictly increasing at index %d: %v", i, xs)
	}
}

// TestBuildBoundariesNoCutsFallsBackToEqualBins covers the "continuous footage"
// path: with no cuts the function returns maxRooms equal time-bins that start at
// 0 and increase monotonically. The bins are evenly spaced (b[i] = i*dur/k).
func TestBuildBoundariesNoCutsFallsBackToEqualBins(t *testing.T) {
	const dur = 12.0
	const maxRooms = 6

	got := buildBoundaries(nil, dur, maxRooms)

	require.Len(t, got, maxRooms, "one bin per room when there are no cuts")
	require.Equal(t, 0.0, got[0], "first bin always starts at 0")
	strictlyIncreasing(t, got)
	require.Less(t, got[len(got)-1], dur, "bins are starts, so all lie before dur")

	// Equal spacing: consecutive gaps are identical.
	step := got[1] - got[0]
	for i := 1; i < len(got); i++ {
		require.InDelta(t, step, got[i]-got[i-1], 1e-9,
			"bins must be equally spaced")
	}
	require.InDelta(t, dur/maxRooms, step, 1e-9, "step should be dur/maxRooms")
}

// TestBuildBoundariesNonPositiveMaxRooms verifies the guard that clamps k to at
// least 1: even with maxRooms <= 0 and no cuts we get a single valid bin at 0
// with no panic and no divide-by-zero.
func TestBuildBoundariesNonPositiveMaxRooms(t *testing.T) {
	for _, maxRooms := range []int{0, -1, -100} {
		require.NotPanics(t, func() {
			got := buildBoundaries(nil, 10.0, maxRooms)
			require.Len(t, got, 1, "clamps to a single bin when maxRooms<=0")
			require.Equal(t, 0.0, got[0])
		}, "maxRooms=%d must not panic", maxRooms)
	}
}

// TestBuildBoundariesValidCutsPrep, verifies that cuts strictly inside
// (0.5, dur-0.5) are kept, and the result is 0 followed by those cuts.
func TestBuildBoundariesValidCutsInside(t *testing.T) {
	const dur = 10.0
	cuts := []float64{2, 5, 8} // all strictly inside (0.5, 9.5)

	got := buildBoundaries(cuts, dur, 6)

	require.Equal(t, []float64{0, 2, 5, 8}, got,
		"result is a leading 0 followed by the valid cuts")
	require.Equal(t, 0.0, got[0])
	strictlyIncreasing(t, got)
}

// TestBuildBoundariesEdgeCutsDropped verifies that cuts on or outside the guard
// band are discarded: c must be strictly > 0.5 and strictly < dur-0.5. The exact
// boundary values 0.5 and dur-0.5 are themselves dropped.
func TestBuildBoundariesEdgeCutsDropped(t *testing.T) {
	const dur = 10.0
	// 0.3, 0.5 -> too early; 9.5, 9.7 -> too late; only 5 survives.
	cuts := []float64{0.3, 0.5, 9.5, 9.7, 5}

	got := buildBoundaries(cuts, dur, 6)

	require.Equal(t, []float64{0, 5}, got, "only the interior cut survives")
}

// TestBuildBoundariesSortsCuts verifies unsorted valid cuts come out ascending.
func TestBuildBoundariesSortsCuts(t *testing.T) {
	const dur = 10.0
	cuts := []float64{8, 2, 5, 3.5}

	got := buildBoundaries(cuts, dur, 6)

	require.Equal(t, 0.0, got[0], "always begins at 0")
	strictlyIncreasing(t, got)
	require.True(t, sort.Float64sAreSorted(got), "output must be sorted ascending")
	require.Equal(t, []float64{0, 2, 3.5, 5, 8}, got)
}

// TestBuildBoundariesShortDurationFiltersCuts verifies that when dur <= 1 the
// guard band (0.5, dur-0.5) is empty or inverted, so every cut is filtered out
// and the function falls back to equal bins.
func TestBuildBoundariesShortDurationFiltersCuts(t *testing.T) {
	const dur = 1.0 // dur-0.5 == 0.5, so (0.5, 0.5) is empty: nothing is valid
	const maxRooms = 3
	cuts := []float64{0.4, 0.6, 0.7} // none can satisfy c>0.5 AND c<0.5

	got := buildBoundaries(cuts, dur, maxRooms)

	require.Len(t, got, maxRooms, "fell back to maxRooms bins")
	require.Equal(t, 0.0, got[0])
	strictlyIncreasing(t, got)
	// None of the original cuts leaked through.
	for _, c := range cuts {
		require.NotContains(t, got, c)
	}
}
