package symbolicator

import gosourcemap "github.com/go-sourcemap/sourcemap"

type GoSourceMapParser struct{}

func (GoSourceMapParser) Parse(sourceMap []byte) (SourceMap, error) {
	consumer, err := gosourcemap.Parse("", sourceMap)
	if err != nil {
		return nil, err
	}
	return goSourceMap{consumer: consumer}, nil
}

type goSourceMap struct {
	consumer *gosourcemap.Consumer
}

func (m goSourceMap) Source(genLine, genCol uint32) (SourceLocation, bool) {
	file, name, line, col, ok := m.consumer.Source(int(genLine)+1, int(genCol))
	if !ok {
		return SourceLocation{}, false
	}
	return SourceLocation{
		File: file,
		Line: uint32(line),
		Col:  uint32(col) + 1,
		Name: name,
	}, true
}
