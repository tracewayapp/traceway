package monitoring

import "github.com/tracewayapp/traceway/backend/v2/app/traceway"

func RecordRecordingUploader(queueDepth, inFlight int, uploadedDelta, droppedDelta, failedDelta uint64) {
	traceway.CaptureMetric("traceway.recordings.queue_depth", float64(queueDepth))
	traceway.CaptureMetric("traceway.recordings.in_flight", float64(inFlight))
	traceway.CaptureMetric("traceway.recordings.uploaded.delta", float64(uploadedDelta))
	traceway.CaptureMetric("traceway.recordings.dropped.delta", float64(droppedDelta))
	traceway.CaptureMetric("traceway.recordings.failed.delta", float64(failedDelta))
}
