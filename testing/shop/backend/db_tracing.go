package main

import (
	"context"
	"database/sql"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var wsRe = regexp.MustCompile(`\s+`)

func collapseWS(q string) string {
	return strings.TrimSpace(wsRe.ReplaceAllString(q, " "))
}

func startDbSpan(ctx context.Context, query string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "db.query", trace.WithAttributes(
		attribute.String("db.system", "sqlite"),
		attribute.String("db.query.text", collapseWS(query)),
	))
}

func queryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	ctx, span := startDbSpan(ctx, query)
	defer span.End()
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return rows, err
}

func queryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	ctx, span := startDbSpan(ctx, query)
	defer span.End()
	return db.QueryRowContext(ctx, query, args...)
}

func execContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	ctx, span := startDbSpan(ctx, query)
	defer span.End()
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return res, err
}
