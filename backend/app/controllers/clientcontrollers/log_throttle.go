package clientcontrollers

import (
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/clientlogs"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/monitoring"
)

func throttleMessages(project *models.Project, projectID uuid.UUID, ip string, request *ReportRequest) (int, *models.ExceptionStackTrace) {
	count := 0
	for _, frame := range request.CollectionFrames {
		for _, row := range frame.StackTraces {
			if row.IsMessage {
				count++
			}
		}
	}
	monitoring.RecordLogsReceived(count)
	if !clientlogs.IsClient(project) {
		return 0, nil
	}
	limiter := clientlogs.Default()
	accepted, flag, saturated := limiter.Allow(projectID, ip, count, time.Now())
	dropped := count - accepted
	monitoring.RecordLogsDropped(dropped, saturated)
	var review *models.ExceptionStackTrace
	for _, frame := range request.CollectionFrames {
		kept := frame.StackTraces[:0]
		for _, row := range frame.StackTraces {
			if !row.IsMessage {
				kept = append(kept, row)
				continue
			}
			if accepted > 0 {
				accepted--
				kept = append(kept, row)
				continue
			}
			if flag && review == nil {
				var session *uuid.UUID
				if row.SessionId != nil {
					if id, err := uuid.Parse(*row.SessionId); err == nil {
						session = &id
					}
				}
				issue := limiter.Review(projectID, ip, dropped, session, row.Attributes)
				review = &issue
			}
		}
		clear(frame.StackTraces[len(kept):])
		frame.StackTraces = kept
	}
	return dropped, review
}
