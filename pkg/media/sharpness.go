package media

import (
	"image"
	_ "image/jpeg" // register JPEG decoder
	"os"
)

// SharpnessJPEG returns a focus score for the JPEG at path: the variance of its
// Laplacian over luminance. Higher = sharper (crisp edges); low = blurry / motion-
// smeared. Used to pick the best hero frame within a segment.
func SharpnessJPEG(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return 0, err
	}
	return Sharpness(img), nil
}

// Sharpness computes the variance of the Laplacian over the image's luminance.
func Sharpness(img image.Image) float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 3 || h < 3 {
		return 0
	}

	// Luminance plane. Fast path for JPEG's native YCbCr (Y is already luma).
	gray := make([]float64, w*h)
	if yc, ok := img.(*image.YCbCr); ok {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				gray[y*w+x] = float64(yc.Y[yc.YOffset(b.Min.X+x, b.Min.Y+y)])
			}
		}
	} else {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				gray[y*w+x] = 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8)
			}
		}
	}

	// Variance of the 4-neighbour Laplacian over interior pixels.
	var sum, sumSq float64
	var n float64
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			c := gray[y*w+x]
			lap := gray[(y-1)*w+x] + gray[(y+1)*w+x] + gray[y*w+x-1] + gray[y*w+x+1] - 4*c
			sum += lap
			sumSq += lap * lap
			n++
		}
	}
	if n == 0 {
		return 0
	}
	mean := sum / n
	return sumSq/n - mean*mean
}
