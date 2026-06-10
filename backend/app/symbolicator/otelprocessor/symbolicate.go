package otelprocessor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/symbolicator"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/jsstack"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

const (
	parsingMethodStructured = "structured_stacktrace_attributes"
	parsingMethodParsed     = "processor_parsed"
)

var errMismatchedLength = errors.New("mismatched stacktrace attribute lengths")
var errUnparseableStackTrace = errors.New("unable to parse stack trace")

type symbolicatorProcessor struct {
	cfg    *Config
	store  *artifactStore
	cache  *resolverCache
	logger *zap.Logger
}

type stackFrame struct {
	fn   string
	url  string
	line int64
	col  int64
}

type frameResult struct {
	fn      string
	url     string
	line    int64
	col     int64
	skipped bool
	err     error
}

func (p *symbolicatorProcessor) processRecord(ctx context.Context, attrs, resource pcommon.Map) {
	if !p.languageAllowed(attrs, resource) {
		return
	}
	stackVal, ok := attrs.Get(p.cfg.StackTraceAttributeKey)
	if !ok {
		return
	}
	originalStack := stackVal.Str()

	buildUUID := ""
	if v, ok := resource.Get(p.cfg.BuildUUIDAttributeKey); ok {
		buildUUID = v.Str()
	}

	frames, structured, parseErr := p.extractFrames(attrs, originalStack)
	method := parsingMethodParsed
	if structured {
		method = parsingMethodStructured
	}
	attrs.PutStr(p.cfg.SymbolicatorParsingMethodAttributeKey, method)

	if parseErr != nil {
		attrs.PutBool(p.cfg.SymbolicatorFailureAttributeKey, true)
		attrs.PutStr(p.cfg.SymbolicatorErrorAttributeKey, parseErr.Error())
		return
	}

	if p.cfg.PreserveStackTrace {
		attrs.PutStr(p.cfg.OriginalStackTraceAttributeKey, originalStack)
		if structured {
			copySlice(attrs, p.cfg.ColumnsAttributeKey, p.cfg.OriginalColumnsAttributeKey)
			copySlice(attrs, p.cfg.FunctionsAttributeKey, p.cfg.OriginalFunctionsAttributeKey)
			copySlice(attrs, p.cfg.LinesAttributeKey, p.cfg.OriginalLinesAttributeKey)
			copySlice(attrs, p.cfg.UrlsAttributeKey, p.cfg.OriginalUrlsAttributeKey)
		}
	}

	results := make([]frameResult, len(frames))
	for i, f := range frames {
		results[i] = p.symbolicateFrame(ctx, f, buildUUID)
	}

	var lines []string
	excType := strAttr(attrs, p.cfg.ExceptionTypeAttributeKey)
	excMessage := strAttr(attrs, p.cfg.ExceptionMessageAttributeKey)
	if excType != "" && excMessage != "" {
		lines = append(lines, excType+": "+excMessage)
	}

	failed := false
	var firstErr error
	for i := range results {
		r := &results[i]
		switch {
		case r.err != nil:
			failed = true
			if firstErr == nil {
				firstErr = r.err
			}
			lines = append(lines, fmt.Sprintf("\tFailed to symbolicate %s at %s:%d:%d: %v", frames[i].fn, frames[i].url, frames[i].line, frames[i].col, r.err))
		case r.skipped:
			lines = append(lines, fmt.Sprintf("    at %s (%s)", r.fn, r.url))
		default:
			lines = append(lines, fmt.Sprintf("    at %s(%s:%d:%d)", r.fn, r.url, r.line, r.col))
		}
	}

	attrs.PutStr(p.cfg.StackTraceAttributeKey, strings.Join(lines, "\n"))
	if structured {
		p.writeFrameSlices(attrs, frames, results)
	}

	attrs.PutBool(p.cfg.SymbolicatorFailureAttributeKey, failed)
	if failed {
		failures := 0
		for i := range results {
			if results[i].err != nil {
				failures++
			}
		}
		err := firstErr
		if failures > 1 {
			err = fmt.Errorf("symbolication failed for some stack frames: %w", firstErr)
		}
		attrs.PutStr(p.cfg.SymbolicatorErrorAttributeKey, err.Error())
	}
	attrs.PutStr("traceway.processor_type", componentType.String())
	attrs.PutStr("traceway.processor_version", processorVersion)
}

func (p *symbolicatorProcessor) languageAllowed(attrs, resource pcommon.Map) bool {
	if len(p.cfg.AllowedLanguages) == 0 {
		return true
	}
	lang := strAttr(attrs, p.cfg.LanguageAttributeKey)
	if lang == "" {
		lang = strAttr(resource, p.cfg.LanguageAttributeKey)
	}
	if lang == "" {
		return false
	}
	for _, allowed := range p.cfg.AllowedLanguages {
		if strings.EqualFold(lang, allowed) {
			return true
		}
	}
	return false
}

func (p *symbolicatorProcessor) extractFrames(attrs pcommon.Map, rawStack string) ([]stackFrame, bool, error) {
	urls, hasUrls := sliceAttr(attrs, p.cfg.UrlsAttributeKey)
	functions, hasFunctions := sliceAttr(attrs, p.cfg.FunctionsAttributeKey)
	lines, hasLines := sliceAttr(attrs, p.cfg.LinesAttributeKey)
	columns, hasColumns := sliceAttr(attrs, p.cfg.ColumnsAttributeKey)

	if hasUrls && hasFunctions && hasLines && hasColumns {
		n := urls.Len()
		if functions.Len() != n || lines.Len() != n || columns.Len() != n {
			return nil, true, errMismatchedLength
		}
		frames := make([]stackFrame, n)
		for i := 0; i < n; i++ {
			frames[i] = stackFrame{
				fn:   functions.At(i).Str(),
				url:  urls.At(i).Str(),
				line: lines.At(i).Int(),
				col:  columns.At(i).Int(),
			}
		}
		return frames, true, nil
	}

	parsed := jsstack.ParseFrames(rawStack)
	if len(parsed) == 0 {
		return nil, false, errUnparseableStackTrace
	}
	frames := make([]stackFrame, len(parsed))
	for i, f := range parsed {
		fn := f.Function
		if fn == "" {
			fn = "?"
		}
		frames[i] = stackFrame{fn: fn, url: f.URL, line: int64(f.Line), col: int64(f.Col)}
	}
	return frames, false, nil
}

func (p *symbolicatorProcessor) symbolicateFrame(ctx context.Context, f stackFrame, buildUUID string) frameResult {
	switch f.url {
	case "", "<anonymous>", "(native)", "[native code]":
		url := f.url
		if url == "" {
			url = "<anonymous>"
		}
		return frameResult{fn: f.fn, url: url, skipped: true}
	}
	if f.line <= 0 || f.col <= 0 || f.line > math.MaxUint32 || f.col > math.MaxUint32 {
		return frameResult{err: fmt.Errorf("line/column out of range: %d:%d", f.line, f.col)}
	}

	resolver, err := p.cache.get(ctx, f.url+"|"+buildUUID, func(ctx context.Context) (*symbolicator.Resolver, error) {
		fetchCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
		defer cancel()
		source, sourceMap, err := p.store.getSourceAndMap(fetchCtx, f.url, buildUUID)
		if err != nil {
			return nil, err
		}
		return symbolicator.NewResolver(sourceMap, source)
	})
	if err != nil {
		return frameResult{err: err}
	}

	frame, ok := resolver.Lookup(uint32(f.line-1), uint32(f.col-1))
	if !ok {
		return frameResult{err: fmt.Errorf("no mapping at %d:%d", f.line, f.col)}
	}
	fn := frame.Fn
	if fn == "" {
		fn = f.fn
	}
	file := frame.File
	if file == "" {
		file = "<unknown>"
	}
	return frameResult{fn: fn, url: file, line: int64(frame.Line), col: int64(frame.Col)}
}

func (p *symbolicatorProcessor) writeFrameSlices(attrs pcommon.Map, frames []stackFrame, results []frameResult) {
	columns := attrs.PutEmptySlice(p.cfg.ColumnsAttributeKey)
	functions := attrs.PutEmptySlice(p.cfg.FunctionsAttributeKey)
	lines := attrs.PutEmptySlice(p.cfg.LinesAttributeKey)
	urls := attrs.PutEmptySlice(p.cfg.UrlsAttributeKey)

	for i := range results {
		r := &results[i]
		switch {
		case r.err != nil:
			columns.AppendEmpty().SetInt(-1)
			functions.AppendEmpty().SetStr("")
			lines.AppendEmpty().SetInt(-1)
			urls.AppendEmpty().SetStr("")
		case r.skipped:
			columns.AppendEmpty().SetInt(frames[i].col)
			functions.AppendEmpty().SetStr(frames[i].fn)
			lines.AppendEmpty().SetInt(frames[i].line)
			urls.AppendEmpty().SetStr(frames[i].url)
		default:
			columns.AppendEmpty().SetInt(r.col)
			functions.AppendEmpty().SetStr(r.fn)
			lines.AppendEmpty().SetInt(r.line)
			urls.AppendEmpty().SetStr(r.url)
		}
	}
}

func strAttr(attrs pcommon.Map, key string) string {
	if v, ok := attrs.Get(key); ok {
		return v.Str()
	}
	return ""
}

func sliceAttr(attrs pcommon.Map, key string) (pcommon.Slice, bool) {
	v, ok := attrs.Get(key)
	if !ok || v.Type() != pcommon.ValueTypeSlice {
		return pcommon.Slice{}, false
	}
	return v.Slice(), true
}

func copySlice(attrs pcommon.Map, from, to string) {
	v, ok := attrs.Get(from)
	if !ok || v.Type() != pcommon.ValueTypeSlice {
		return
	}
	v.Slice().CopyTo(attrs.PutEmptySlice(to))
}
