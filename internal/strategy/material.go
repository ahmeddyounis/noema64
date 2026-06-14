package strategy

import (
	"fmt"
	"math"

	"github.com/ahmedyounis/noema64/internal/chesscore"
)

// MaterialContextSummary turns the engine's signed material balance into a
// user-facing sentence. MaterialBalance is white-minus-black centipawns.
func MaterialContextSummary(features chesscore.FeatureSummary) string {
	balance := features.MaterialBalance
	if absInt(balance) < 50 {
		return "Material is equal."
	}
	leader := "White"
	if balance < 0 {
		leader = "Black"
	}
	return fmt.Sprintf("%s is ahead by %s of material.", leader, materialPointText(absInt(balance)))
}

func materialPointText(cp int) string {
	points := float64(cp) / 100.0
	if cp%100 == 0 {
		whole := cp / 100
		if whole == 1 {
			return "1 point"
		}
		return fmt.Sprintf("%d points", whole)
	}
	rounded := math.Round(points*10) / 10
	if rounded == 1 {
		return "about 1 point"
	}
	return fmt.Sprintf("about %.1f points", rounded)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
