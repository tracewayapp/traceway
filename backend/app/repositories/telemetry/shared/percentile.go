package shared

func ComputePercentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	idx := p * float64(n-1)
	lower := int(idx)
	frac := idx - float64(lower)
	if lower+1 >= n {
		return sorted[lower]
	}
	return sorted[lower]*(1-frac) + sorted[lower+1]*frac
}
