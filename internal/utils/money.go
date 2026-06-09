package utils

import "math"

func ToKopecks(rub float64) int64 { return int64(math.Round(rub * 100)) }

func ToRub(kopecks int64) float64 { return float64(kopecks) / 100 }
