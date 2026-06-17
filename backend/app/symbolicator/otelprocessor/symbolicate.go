package otelprocessor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/symbolicator/dart"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/ios"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap/jsstack"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/twcache"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

const (
	parsingMethodStructured = "structured_stacktrace_attributes"
	parsingMethodParsed     = "processor_parsed"
	maxDartFrames           = 50
)

var errMismatchedLength = errors.New("mismatched stacktrace attribute lengths")
var errUnparseableStackTrace = errors.New("unable to parse stack trace")
var errMissingDartBuild = errors.New("dart trace missing build_id or arch")
var errNoIOSSymbols = errors.New("no ios symbols found")

type symbolicatorProcessor struct {
	cfg    *Config
	store  *artifactStore
	cache  *twcache.Cache
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

	lang := strAttr(attrs, p.cfg.LanguageAttributeKey)
	if lang == "" {
		lang = strAttr(resource, p.cfg.LanguageAttributeKey)
	}
	if ios.IsIOSLanguage(lang) {
		p.symbolicateIOSTrace(ctx, attrs, resource, originalStack)
		return
	}

	if dart.IsNonSymbolic(originalStack) {
		p.symbolicateDartTrace(ctx, attrs, originalStack)
		return
	}

	if ios.IsIOSTrace(originalStack) || ios.IsHoneycombTrace(originalStack) {
		p.symbolicateIOSTrace(ctx, attrs, resource, originalStack)
		return
	}

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
	p.putProcessorMeta(attrs)
}

func (p *symbolicatorProcessor) putProcessorMeta(attrs pcommon.Map) {
	attrs.PutStr("traceway.processor_type", componentType.String())
	attrs.PutStr("traceway.processor_version", processorVersion)
}

func (p *symbolicatorProcessor) symbolicateDartTrace(ctx context.Context, attrs pcommon.Map, rawStack string) {
	attrs.PutStr(p.cfg.SymbolicatorParsingMethodAttributeKey, parsingMethodParsed)
	if p.cfg.PreserveStackTrace {
		attrs.PutStr(p.cfg.OriginalStackTraceAttributeKey, rawStack)
	}

	trace := dart.ParseTrace(rawStack)
	arch := trace.Arch
	if arch == "" {
		arch = p.cfg.DartDefaultArch
	}

	fail := func(err error) {
		attrs.PutStr(p.cfg.StackTraceAttributeKey, p.renderDartTrace(attrs, trace, nil))
		attrs.PutBool(p.cfg.SymbolicatorFailureAttributeKey, true)
		attrs.PutStr(p.cfg.SymbolicatorErrorAttributeKey, err.Error())
		p.putProcessorMeta(attrs)
	}

	if len(trace.Frames) == 0 {
		fail(errUnparseableStackTrace)
		return
	}
	if trace.BuildID == "" || arch == "" {
		fail(errMissingDartBuild)
		return
	}

	key := flatCacheKey(p.store.dartSymbolsKey(trace.BuildID, arch))
	if p.cache.IsNegative(key) {
		fail(fmt.Errorf("failed to find dart symbols for build %s/%s", trace.BuildID, arch))
		return
	}

	data, done, err := p.cache.Get(ctx, key, func(ctx context.Context) ([]byte, error) {
		fetchCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
		defer cancel()
		elf, err := p.store.getDartSymbols(fetchCtx, trace.BuildID, arch)
		if err != nil {
			return nil, err
		}
		return dart.BuildFlat(elf)
	})
	if err != nil {
		fail(err)
		return
	}
	defer done()

	attrs.PutStr(p.cfg.StackTraceAttributeKey, p.renderDartTrace(attrs, trace, data))
	attrs.PutBool(p.cfg.SymbolicatorFailureAttributeKey, false)
	p.putProcessorMeta(attrs)
}

func (p *symbolicatorProcessor) renderDartTrace(attrs pcommon.Map, trace dart.StackTrace, data []byte) string {
	var b strings.Builder
	excType := strAttr(attrs, p.cfg.ExceptionTypeAttributeKey)
	excMessage := strAttr(attrs, p.cfg.ExceptionMessageAttributeKey)
	if excType != "" && excMessage != "" {
		fmt.Fprintf(&b, "%s: %s\n", excType, excMessage)
	}
	n := 0
	for _, f := range trace.Frames {
		if n >= maxDartFrames {
			break
		}
		var resolved []dart.SymFrame
		if data != nil {
			resolved = dart.LookupFlat(data, f)
		}
		if len(resolved) == 0 {
			fmt.Fprintf(&b, "#%d  %s+%x\n", n, dart.InstructionSymbol(f.Section), f.Offset)
			n++
			continue
		}
		for _, sf := range resolved {
			if n >= maxDartFrames {
				break
			}
			fmt.Fprintf(&b, "#%d  %s (%s)\n", n, sf.Function, sf.Location())
			n++
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (p *symbolicatorProcessor) symbolicateIOSTrace(ctx context.Context, attrs, resource pcommon.Map, rawStack string) {
	trace := ios.ParseTrace(rawStack)
	if len(trace.Frames) == 0 {
		buildUUID := strAttr(resource, p.cfg.IOSBuildUUIDAttributeKey)
		appExecutable := strAttr(resource, p.cfg.AppExecutableAttributeKey)
		trace = ios.ParseHoneycombTrace(rawStack, buildUUID, appExecutable)
	}
	if len(trace.Frames) == 0 {
		return
	}

	attrs.PutStr(p.cfg.SymbolicatorParsingMethodAttributeKey, parsingMethodParsed)
	if p.cfg.PreserveStackTrace {
		attrs.PutStr(p.cfg.OriginalStackTraceAttributeKey, rawStack)
	}

	arch := trace.Arch
	if arch == "" {
		arch = p.cfg.IOSDefaultArch
	}
	if arch == "" {
		arch = "arm64"
	}

	dataByUUID := map[string][]byte{}
	var dones []func()
	defer func() {
		for _, d := range dones {
			d()
		}
	}()
	var resolveErr error
	resolved := false
	resolver := func(debugID string) []byte {
		if d, ok := dataByUUID[debugID]; ok {
			return d
		}
		key := flatCacheKey(p.store.iosSymbolsKey(debugID))
		if p.cache.IsNegative(key) {
			dataByUUID[debugID] = nil
			if resolveErr == nil {
				resolveErr = fmt.Errorf("failed to find ios symbols for %s", debugID)
			}
			return nil
		}
		data, done, err := p.cache.Get(ctx, key, func(ctx context.Context) ([]byte, error) {
			fetchCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
			defer cancel()
			dsym, err := p.store.getIOSDsym(fetchCtx, debugID)
			if err != nil {
				return nil, err
			}
			return ios.BuildFlat(dsym, debugID, arch)
		})
		if err != nil {
			dataByUUID[debugID] = nil
			if resolveErr == nil {
				resolveErr = err
			}
			return nil
		}
		dones = append(dones, done)
		dataByUUID[debugID] = data
		resolved = true
		return data
	}

	attrs.PutStr(p.cfg.StackTraceAttributeKey, p.renderIOSTrace(attrs, trace, resolver))
	if resolved {
		attrs.PutBool(p.cfg.SymbolicatorFailureAttributeKey, false)
	} else {
		if resolveErr == nil {
			resolveErr = errNoIOSSymbols
		}
		attrs.PutBool(p.cfg.SymbolicatorFailureAttributeKey, true)
		attrs.PutStr(p.cfg.SymbolicatorErrorAttributeKey, resolveErr.Error())
	}
	p.putProcessorMeta(attrs)
}

func (p *symbolicatorProcessor) renderIOSTrace(attrs pcommon.Map, trace ios.StackTrace, resolver func(string) []byte) string {
	preamble := ""
	if excType := strAttr(attrs, p.cfg.ExceptionTypeAttributeKey); excType != "" {
		if excMessage := strAttr(attrs, p.cfg.ExceptionMessageAttributeKey); excMessage != "" {
			preamble = excType + ": " + excMessage
		}
	}
	return ios.RenderResolved(trace, preamble, func(uuid string, off uint64) []ios.SymFrame {
		data := resolver(uuid)
		if data == nil {
			return nil
		}
		return ios.LookupFlat(data, off)
	})
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

	key := cacheKey(f.url, buildUUID)
	if p.cache.IsNegative(key) {
		return frameResult{err: fmt.Errorf("no source map for %s", f.url)}
	}
	data, done, err := p.cache.Get(ctx, key, func(ctx context.Context) ([]byte, error) {
		fetchCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
		defer cancel()
		source, sourceMap, err := p.store.getSourceAndMap(fetchCtx, f.url, buildUUID)
		if err != nil {
			return nil, err
		}
		return sourcemap.BuildTW(sourceMap, source)
	})
	if err != nil {
		return frameResult{err: err}
	}
	defer done()

	frame, ok := sourcemap.LookupTW(data, uint32(f.line-1), uint32(f.col-1))
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
