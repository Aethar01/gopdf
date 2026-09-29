package viewer

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

func TestLoadFontFileSupportsLargeTTC(t *testing.T) {
	const numFonts = 257

	fontData := append([]byte(nil), goregular.TTF...)
	prefixLen := 12 + 4*numFonts
	for i := 0; i < int(binary.BigEndian.Uint16(fontData[4:6])); i++ {
		offsetPos := 12 + i*16 + 8
		offset := binary.BigEndian.Uint32(fontData[offsetPos : offsetPos+4])
		binary.BigEndian.PutUint32(fontData[offsetPos:offsetPos+4], offset+uint32(prefixLen))
	}

	ttc := make([]byte, prefixLen+len(fontData))
	copy(ttc[:4], "ttcf")
	binary.BigEndian.PutUint32(ttc[4:8], 0x00010000)
	binary.BigEndian.PutUint32(ttc[8:12], numFonts)
	for i := 0; i < numFonts; i++ {
		binary.BigEndian.PutUint32(ttc[12+i*4:16+i*4], uint32(prefixLen))
	}
	copy(ttc[prefixLen:], fontData)

	if _, err := opentype.ParseCollection(ttc); err == nil {
		t.Fatal("expected standard collection parser to reject more than 256 fonts")
	}

	path := filepath.Join(t.TempDir(), "large.ttc")
	if err := os.WriteFile(path, ttc, 0o600); err != nil {
		t.Fatal(err)
	}

	small, err := loadFontFile(path, 12)
	if err != nil {
		t.Fatalf("load 12px font: %v", err)
	}
	defer closeFontFace(small)

	large, err := loadFontFile(path, 36)
	if err != nil {
		t.Fatalf("load 36px font: %v", err)
	}
	defer closeFontFace(large)

	if small.Metrics().Height >= large.Metrics().Height {
		t.Fatalf("font size did not scale: 12px height=%v 36px height=%v", small.Metrics().Height, large.Metrics().Height)
	}
}

func TestFileFontMeasuresLikeItsUncachedFace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "regular.ttf")
	if err := os.WriteFile(path, goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	face, err := loadFontFile(path, 14)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFontFace(face)
	const s = "AVAWaY To, kerned text"
	want := measureText(face.(*fileBackedFontFace).Face, s)
	for range 2 { // the second pass reads the cache
		if got := measureText(face, s); got != want {
			t.Fatalf("measured %d, want %d", got, want)
		}
	}
}

func TestRuneIndexAtMatchesPrefixMeasurement(t *testing.T) {
	fnt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	face, err := opentype.NewFace(fnt, &opentype.FaceOptions{Size: 14, DPI: 72})
	if err != nil {
		t.Fatal(err)
	}
	const s = "AVAWaY To, kerned é text"
	runes := []rune(s)
	want := func(x float64) int {
		for i := range runes {
			left, right := measureText(face, string(runes[:i])), measureText(face, string(runes[:i+1]))
			if x < float64(left+right)/2 {
				return i
			}
		}
		return len(runes)
	}
	for x := -2.0; x < float64(measureText(face, s))+4; x += 0.5 {
		if got, want := runeIndexAt(face, s, x), want(x); got != want {
			t.Fatalf("runeIndexAt(%v) = %d, want %d", x, got, want)
		}
	}
}
