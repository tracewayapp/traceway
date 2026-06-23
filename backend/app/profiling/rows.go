package profiling

import (
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func BuildRows(projectId uuid.UUID, decoded []Decoded) ([]models.ProfileStack, []models.ProfileSample, []models.Profile) {
	var totalStacks, totalSamples int
	for _, d := range decoded {
		totalStacks += len(d.Stacks)
		totalSamples += len(d.Samples)
	}
	stacks := make([]models.ProfileStack, 0, totalStacks)
	samples := make([]models.ProfileSample, 0, totalSamples)
	var profiles []models.Profile

	for _, d := range decoded {
		for _, s := range d.Stacks {
			stacks = append(stacks, models.ProfileStack{
				ProjectId:   projectId,
				ServiceName: d.Meta.ServiceName,
				StackHash:   s.Hash,
				Stack:       s.Frames,
				LastSeen:    d.Meta.End,
			})
		}

		byType := make(map[string][]Sample)
		for _, s := range d.Samples {
			byType[s.Type] = append(byType[s.Type], s)
		}

		for typ, group := range byType {
			profileId := uuid.New()
			var total int64
			for _, s := range group {
				total += s.Value
				samples = append(samples, models.ProfileSample{
					ProjectId:   projectId,
					ProfileId:   profileId,
					ServiceName: d.Meta.ServiceName,
					Type:        typ,
					Start:       d.Meta.Start,
					End:         d.Meta.End,
					StackHash:   s.StackHash,
					Value:       s.Value,
					ServerName:  d.Meta.ServerName,
					AppVersion:  d.Meta.AppVersion,
				})
			}
			profiles = append(profiles, models.Profile{
				Id:          profileId,
				ProjectId:   projectId,
				RecordedAt:  d.Meta.Start,
				Duration:    d.Meta.End.Sub(d.Meta.Start),
				ServiceName: d.Meta.ServiceName,
				ProfileType: typ,
				SampleCount: uint64(len(group)),
				TotalValue:  total,
				ServerName:  d.Meta.ServerName,
				AppVersion:  d.Meta.AppVersion,
			})
		}
	}

	return stacks, samples, profiles
}
