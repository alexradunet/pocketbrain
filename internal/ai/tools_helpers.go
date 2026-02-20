package ai

import (
	"fmt"
	"math"
)

// displayFolder returns a human-friendly folder label.
func displayFolder(folder string) string {
	if folder == "" {
		return "workspace root"
	}
	return folder
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	k := float64(1024)
	sizes := []string{"B", "KB", "MB", "GB"}
	i := int(math.Floor(math.Log(float64(bytes)) / math.Log(k)))
	if i >= len(sizes) {
		i = len(sizes) - 1
	}
	val := float64(bytes) / math.Pow(k, float64(i))
	return fmt.Sprintf("%.1f %s", val, sizes[i])
}
