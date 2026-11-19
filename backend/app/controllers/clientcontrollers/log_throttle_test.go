package clientcontrollers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/models/clientmodels"
)

func TestNativeMessagesThrottledWithoutDroppingErrors(t *testing.T) {
	request := ReportRequest{CollectionFrames: []*clientmodels.CollectionFrame{{}, {}}}
	for _, frame := range request.CollectionFrames {
		for range 600 {
			frame.StackTraces = append(frame.StackTraces, &clientmodels.ClientExceptionStackTrace{IsMessage: true})
		}
		frame.StackTraces = append(frame.StackTraces, &clientmodels.ClientExceptionStackTrace{StackTrace: "real error"})
	}
	dropped, review := throttleMessages(&models.Project{Framework: "flutter"}, uuid.New(), "192.0.2.1", &request)
	if dropped != 200 || review == nil {
		t.Fatalf("dropped=%d review=%v", dropped, review)
	}
	messages, errors := 0, 0
	for _, frame := range request.CollectionFrames {
		for _, row := range frame.StackTraces {
			if row.IsMessage {
				messages++
			} else {
				errors++
			}
		}
	}
	if messages != 1000 || errors != 2 {
		t.Fatalf("messages=%d errors=%d", messages, errors)
	}
}
