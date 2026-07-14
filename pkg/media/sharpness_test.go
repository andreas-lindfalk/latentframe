package media

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testDim  = 64 // image side length in pixels
	testCell = 8  // checkerboard cell size; must exceed the blur radius so a
	// 3x3 box blur softens edges rather than washing them out entirely.
)

// uniformImage returns a solid mid-gray image (no edges anywhere).
func uniformImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, testDim, testDim))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{128, 128, 128, 255}},
		image.Point{}, draw.Src)
	return img
}

// checkerboardImage returns a black/white checkerboard: maximum-contrast edges
// at every cell boundary.
func checkerboardImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, testDim, testDim))
	for y := 0; y < testDim; y++ {
		for x := 0; x < testDim; x++ {
			v := uint8(0)
			if ((x/testCell)+(y/testCell))%2 == 0 {
				v = 255
			}
			img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

// boxBlur returns a 3x3 box-blurred (radius 1) copy of src's luminance. Edges
// are clamped. The output is a gray RGBA image so its luminance equals the blur.
func boxBlur(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	lum := func(x, y int) float64 {
		if x < 0 {
			x = 0
		} else if x >= w {
			x = w - 1
		}
		if y < 0 {
			y = 0
		} else if y >= h {
			y = h - 1
		}
		c := src.RGBAAt(x, y)
		return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
	}

	out := image.NewRGBA(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum float64
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					sum += lum(x+dx, y+dy)
				}
			}
			v := uint8(sum / 9)
			out.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return out
}

// TestSharpnessOrdering asserts the core invariant of the variance-of-Laplacian
// focus score: uniform (no edges) < box-blurred edges < sharp edges.
func TestSharpnessOrdering(t *testing.T) {
	uniform := Sharpness(uniformImage())
	checker := checkerboardImage()
	sharp := Sharpness(checker)
	blurred := Sharpness(boxBlur(checker))

	// A perfectly flat image has zero Laplacian variance.
	require.InDelta(t, 0.0, uniform, 1e-9, "uniform image should score ~0")

	// Both edge images register some focus energy.
	require.Greater(t, blurred, 0.0, "blurred edges still carry some energy")
	require.Greater(t, sharp, 0.0, "sharp edges carry energy")

	// The ordering is the whole point of a focus score.
	require.Greater(t, blurred, uniform, "blurred must beat a flat image")
	require.Greater(t, sharp, blurred, "sharp must beat its blurred copy")
}

// TestSharpnessTinyImage documents the guard: images smaller than 3x3 have no
// interior pixels and score 0 rather than panicking.
func TestSharpnessTinyImage(t *testing.T) {
	tiny := image.NewRGBA(image.Rect(0, 0, 2, 2))
	require.Equal(t, 0.0, Sharpness(tiny))
}

// TestSharpnessJPEG exercises the file+decode path (which yields *image.YCbCr and
// thus the luma fast path). A JPEG-encoded checkerboard must score clearly higher
// than a JPEG-encoded flat gray image.
func TestSharpnessJPEG(t *testing.T) {
	writeJPEG := func(t *testing.T, img image.Image) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "sharp-*.jpg")
		require.NoError(t, err)
		defer f.Close()
		require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 95}))
		return f.Name()
	}

	checkerPath := writeJPEG(t, checkerboardImage())
	uniformPath := writeJPEG(t, uniformImage())

	checkerScore, err := SharpnessJPEG(checkerPath)
	require.NoError(t, err)
	uniformScore, err := SharpnessJPEG(uniformPath)
	require.NoError(t, err)

	require.Greater(t, checkerScore, 0.0, "encoded checkerboard should score > 0")
	require.Greater(t, checkerScore, uniformScore,
		"sharp JPEG must out-score a flat JPEG")
}

// TestSharpnessJPEGMissingFile confirms a read error is surfaced, not swallowed.
func TestSharpnessJPEGMissingFile(t *testing.T) {
	_, err := SharpnessJPEG("/no/such/file.jpg")
	require.Error(t, err)
}
