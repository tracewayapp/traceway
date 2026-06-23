package profiling

import (
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func BuildRows(d Decoded, projectId uuid.UUID) ([]models.ProfileStack, []models.ProfileSample, []models.Profile) {
	stacks := make([]models.ProfileStack, 0, len(d.Stacks))
	for _, s := range d.Stacks {
		stacks = append(stacks, models.ProfileStack{
			ProjectId:   projectId,
			ServiceName: d.Meta.ServiceName,
			StackHash:   s.Hash,
			Stack:       s.Frames,
			LastSeen:    d.Meta.End,
		})
	}

	traceId, spanId := "", ""
	if d.Meta.TraceId != nil {
		traceId = *d.Meta.TraceId
	}
	if d.Meta.SpanId != nil {
		spanId = *d.Meta.SpanId
	}

	profileByType := map[string]*models.Profile{}
	var typeOrder []string
	samples := make([]models.ProfileSample, 0, len(d.Samples))
	for _, s := range d.Samples {
		p := profileByType[s.Type]
		if p == nil {
			p = &models.Profile{
				Id:          uuid.New(),
				ProjectId:   projectId,
				RecordedAt:  d.Meta.Start,
				Duration:    d.Meta.End.Sub(d.Meta.Start),
				ServiceName: d.Meta.ServiceName,
				ProfileType: s.Type,
				ServerName:  d.Meta.ServerName,
				AppVersion:  d.Meta.AppVersion,
				Attributes:  d.Meta.Attributes,
				TraceId:     traceId,
				SpanId:      spanId,
			}
			profileByType[s.Type] = p
			typeOrder = append(typeOrder, s.Type)
		}
		p.SampleCount++
		p.TotalValue += s.Value
		samples = append(samples, models.ProfileSample{
			ProjectId:   projectId,
			ProfileId:   p.Id,
			ServiceName: d.Meta.ServiceName,
			Type:        s.Type,
			Start:       d.Meta.Start,
			End:         d.Meta.End,
			StackHash:   s.StackHash,
			Value:       s.Value,
			ServerName:  d.Meta.ServerName,
			AppVersion:  d.Meta.AppVersion,
			TraceId:     traceId,
			SpanId:      spanId,
		})
	}

	profiles := make([]models.Profile, 0, len(typeOrder))
	for _, t := range typeOrder {
		profiles = append(profiles, *profileByType[t])
	}

	return stacks, samples, profiles
}
