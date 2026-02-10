package repositories

import (
	"fmt"
	"math"
	"time"
)

func computeImpactReason(
	total, satisfiedCount, toleratingCount, badCount, clientErrorCount uint64,
	p99Ns float64,
	offsetMs uint32,
) string {
	if total == 0 {
		return "Endpoint is healthy"
	}

	totalF := float64(total)
	badF := float64(badCount)
	clientF := float64(clientErrorCount)
	offsetNs := float64(offsetMs) * 1_000_000

	// 1. Inverted Apdex
	apdexScore := 1.0 - (float64(satisfiedCount)+float64(toleratingCount)*0.5)/totalF

	// 2. Error Rate Floor
	badRate := badF / totalF
	var errorRateScore float64
	switch {
	case badRate > 0.33:
		errorRateScore = 0.75
	case badRate > 0.20:
		errorRateScore = 0.50
	case badRate > 0.10:
		errorRateScore = 0.25
	}

	// 3. P99 Floor (offset-adjusted)
	adjustedP99 := p99Ns - offsetNs
	var p99Score float64
	switch {
	case adjustedP99 > 8_000_000_000:
		p99Score = 0.75
	case adjustedP99 > 6_000_000_000:
		p99Score = 0.50
	case adjustedP99 > 3_000_000_000:
		p99Score = 0.25
	}

	// 4. Client Error Floor (only when count > 10)
	var clientErrorScore float64
	if total > 10 {
		clientRate := clientF / totalF
		switch {
		case clientRate > 0.50:
			clientErrorScore = 0.75
		case clientRate > 0.25:
			clientErrorScore = 0.50
		}
	}

	// 5. Volume-Aware Error Floor
	var volumeScore float64
	switch {
	case badRate > 0.10 && badCount >= 500:
		volumeScore = 0.75
	case badRate > 0.10 && badCount >= 50:
		volumeScore = 0.50
	case badRate > 0.05 && badCount >= 2000:
		volumeScore = 0.75
	case badRate > 0.05 && badCount >= 500:
		volumeScore = 0.50
	case badRate > 0.05 && badCount >= 50:
		volumeScore = 0.25
	case badRate > 0.01 && badCount >= 10000:
		volumeScore = 0.75
	case badRate > 0.01 && badCount >= 2000:
		volumeScore = 0.50
	case badRate > 0.01 && badCount >= 500:
		volumeScore = 0.25
	}

	maxScore := math.Max(apdexScore, math.Max(errorRateScore, math.Max(p99Score, math.Max(clientErrorScore, volumeScore))))

	if maxScore < 0.25 {
		return "Endpoint is healthy"
	}

	// Priority when tied: P99 > Volume > Client Error > Error Rate > Apdex
	switch {
	case p99Score >= maxScore:
		p99Duration := time.Duration(p99Ns)
		return fmt.Sprintf("P99 latency is %s", formatDurationHuman(p99Duration))
	case volumeScore >= maxScore:
		return fmt.Sprintf("%s errors with %.1f%% error rate", formatCount(badCount), badRate*100)
	case clientErrorScore >= maxScore:
		return fmt.Sprintf("%.0f%% of requests returning 4xx errors", clientF/totalF*100)
	case errorRateScore >= maxScore:
		return fmt.Sprintf("%.0f%% of requests are slow or failing", badRate*100)
	default:
		return fmt.Sprintf("%.0f%% of requests are slow or failing", (1-float64(satisfiedCount)/totalF)*100)
	}
}

func formatDurationHuman(d time.Duration) string {
	if d >= time.Second {
		secs := d.Seconds()
		if secs >= 10 {
			return fmt.Sprintf("%.0fs", secs)
		}
		return fmt.Sprintf("%.1fs", secs)
	}
	return fmt.Sprintf("%.0fms", float64(d)/float64(time.Millisecond))
}

func formatCount(n uint64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%s", addCommas(n))
	}
	return fmt.Sprintf("%d", n)
}

func addCommas(n uint64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
