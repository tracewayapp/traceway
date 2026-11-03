package sdkconfig

func mergePtr[T any](dst **T, src *T) {
	if src != nil {
		v := *src
		*dst = &v
	}
}

// Merge lays src over dst: set fields replace, triggers merge by id (a trigger without steps patches the fields it
// sets), and captures append.
func Merge(dst *Settings, src Settings) {
	dst.SchemaVersion = SchemaVersion
	mergePtr(&dst.Enabled, src.Enabled)
	if src.Signals != nil {
		if dst.Signals == nil {
			dst.Signals = &Signals{}
		}
		srcFields := map[string]*SignalSettings{}
		src.Signals.each(func(name string, field **SignalSettings) { srcFields[name] = *field })
		dst.Signals.each(func(name string, field **SignalSettings) {
			source := srcFields[name]
			if source == nil {
				return
			}
			if *field == nil {
				*field = &SignalSettings{}
			}
			mergePtr(&(*field).Mode, source.Mode)
			mergePtr(&(*field).SampleRate, source.SampleRate)
			mergePtr(&(*field).MinSeverity, source.MinSeverity)
		})
	}
	if src.Sessions != nil {
		if dst.Sessions == nil {
			dst.Sessions = &SessionSettings{}
		}
		mergePtr(&dst.Sessions.Enabled, src.Sessions.Enabled)
		mergePtr(&dst.Sessions.BaselineSampleRate, src.Sessions.BaselineSampleRate)
	}
	if src.Replay != nil {
		if dst.Replay == nil {
			dst.Replay = &ReplaySettings{}
		}
		mergePtr(&dst.Replay.Mode, src.Replay.Mode)
		mergePtr(&dst.Replay.SessionSampleRate, src.Replay.SessionSampleRate)
		mergePtr(&dst.Replay.MaskAllInputs, src.Replay.MaskAllInputs)
	}
	if src.Buffer != nil {
		if dst.Buffer == nil {
			dst.Buffer = &BufferSettings{}
		}
		mergePtr(&dst.Buffer.MaxBytes, src.Buffer.MaxBytes)
		mergePtr(&dst.Buffer.MaxAgeSeconds, src.Buffer.MaxAgeSeconds)
		mergePtr(&dst.Buffer.ReplayMaxBytes, src.Buffer.ReplayMaxBytes)
		mergePtr(&dst.Buffer.Persist, src.Buffer.Persist)
	}
	for _, t := range src.Triggers {
		dst.Triggers = mergeTrigger(dst.Triggers, t)
	}
	dst.Captures = append(dst.Captures, src.Captures...)
}

func mergeTrigger(triggers []Trigger, t Trigger) []Trigger {
	for i := range triggers {
		if triggers[i].Id != t.Id {
			continue
		}
		existing := &triggers[i]
		mergePtr(&existing.Name, t.Name)
		mergePtr(&existing.Enabled, t.Enabled)
		mergePtr(&existing.SampleRate, t.SampleRate)
		mergePtr(&existing.CooldownSeconds, t.CooldownSeconds)
		mergePtr(&existing.MaxPerSession, t.MaxPerSession)
		if len(t.Steps) > 0 {
			existing.Steps = t.Steps
			existing.Actions = t.Actions
		}
		return triggers
	}
	return append(triggers, t)
}

// ApplyPermission switches off what the organization's plan would drop at ingest anyway, so clients stop sending
// it instead of having it discarded. The same signals go into Limits, so a capture, which otherwise brings back
// signals rules switched off, cannot bring these back.
func ApplyPermission(s *Settings, exceptions, data, replay bool) {
	off := ModeOff
	limit := func(name string) {
		if s.Limits == nil {
			s.Limits = &Limits{}
		}
		if name == "replay" {
			s.Limits.Replay = &off
			return
		}
		if s.Limits.Signals == nil {
			s.Limits.Signals = map[string]Mode{}
		}
		s.Limits.Signals[name] = ModeOff
	}
	if !replay {
		if s.Replay == nil {
			s.Replay = &ReplaySettings{}
		}
		s.Replay.Mode = &off
		limit("replay")
	}
	if exceptions && data {
		return
	}
	if s.Signals == nil {
		s.Signals = &Signals{}
	}
	s.Signals.each(func(name string, field **SignalSettings) {
		isIssue := name == "errors" || name == "messages"
		if (isIssue && exceptions) || (!isIssue && data) {
			return
		}
		if *field == nil {
			*field = &SignalSettings{}
		}
		(*field).Mode = &off
		limit(name)
	})
}
