package challenge

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func BenchmarkCanvasProof(b *testing.B) {
	img := image.NewNRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
	for y := 10; y < 28; y++ {
		for x := 10; x < 50; x++ {
			img.Set(x, y, color.NRGBA{A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		b.Fatal(err)
	}
	proof := canvasDataPrefix + base64.StdEncoding.EncodeToString(buf.Bytes())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !validCanvasProof(proof) {
			b.Fatal("valid PNG was rejected")
		}
	}
}
