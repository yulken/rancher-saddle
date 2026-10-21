package views

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBanner(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		ok    bool
		lines []string
	}{
		{"empty string", "", false, nil},
		{"whitespace only", "   \n\t\n", false, nil},
		{"single line", "hello", true, []string{"hello"}},
		{"multi line", "a\nb\nc", true, []string{"a", "b", "c"}},
		{"trailing newline trimmed", "a\nb\n", true, []string{"a", "b"}},
		{"crlf trailing trimmed", "a\nb\r\n", true, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, ok := parseBanner(tt.raw)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.lines, lines)
		})
	}
}

func TestLoadBannerFrom_NoFile_NotFound(t *testing.T) {
	lines, found := loadBannerFrom([]string{filepath.Join(t.TempDir(), "nope.txt")})
	assert.False(t, found)
	assert.Nil(t, lines)
}

func TestLoadBannerFrom_CustomFileWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "banner.txt")
	art := "LINE 1\nLINE 2"
	assert.NoError(t, os.WriteFile(path, []byte(art), 0600))

	lines, found := loadBannerFrom([]string{path})
	assert.True(t, found)
	assert.Equal(t, []string{"LINE 1", "LINE 2"}, lines)
}

func TestLoadBannerFrom_EmptyFileFallsThrough(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.txt")
	good := filepath.Join(dir, "good.txt")
	assert.NoError(t, os.WriteFile(empty, []byte("  \n"), 0600))
	assert.NoError(t, os.WriteFile(good, []byte("ART"), 0600))

	lines, found := loadBannerFrom([]string{empty, good})
	assert.True(t, found)
	assert.Equal(t, []string{"ART"}, lines)
}

func TestLoadBannerFrom_PriorityOrder(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")
	assert.NoError(t, os.WriteFile(first, []byte("FIRST"), 0600))
	assert.NoError(t, os.WriteFile(second, []byte("SECOND"), 0600))

	lines, found := loadBannerFrom([]string{first, second})
	assert.True(t, found)
	assert.Equal(t, []string{"FIRST"}, lines)
}

func TestLoadBanner_UsesEmbeddedWhenNoOverride(t *testing.T) {
	// With no custom override file present, the embedded art is the default.
	lines, source := loadBanner()
	assert.NotEmpty(t, lines)
	assert.Contains(t, []bannerSource{bannerEmbedded, bannerFiglet}, source)
}
