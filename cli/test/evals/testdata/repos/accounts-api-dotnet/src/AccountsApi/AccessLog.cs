using System.Collections;
using System.Diagnostics;

namespace Accounts.Api;

public static class AccessLog
{
    // One line per request ("GET /api/accounts/42 200 3ms"), inside the request's trace.
    public static IApplicationBuilder UseAccessLog(this IApplicationBuilder app)
    {
        var logger = app.ApplicationServices.GetRequiredService<ILoggerFactory>().CreateLogger("Accounts.Api.AccessLog");
        return app.Use(async (context, next) =>
        {
            var started = Stopwatch.GetTimestamp();
            var status = StatusCodes.Status500InternalServerError;
            try
            {
                await next(context);
                status = context.Response.StatusCode;
            }
            finally
            {
                var entry = new AccessLogEntry(
                    context.Request.Method,
                    context.Request.Path.Value ?? "",
                    (context.GetEndpoint() as RouteEndpoint)?.RoutePattern.RawText ?? "",
                    status,
                    (long)Stopwatch.GetElapsedTime(started).TotalMilliseconds);
                logger.Log(LogLevel.Information, default, entry, null, static (e, _) => e.ToString());
            }
        });
    }
}

// The attributes of an access log line; OpenTelemetry exports them as the
// record's attributes and the formatted line as its body.
public sealed class AccessLogEntry(string method, string path, string route, int status, long durationMs)
    : IReadOnlyList<KeyValuePair<string, object?>>
{
    public int Count => 5;

    public KeyValuePair<string, object?> this[int index] => index switch
    {
        0 => new("http.request.method", method),
        1 => new("url.path", path),
        2 => new("http.route", route),
        3 => new("http.response.status_code", status),
        4 => new("duration_ms", durationMs),
        _ => throw new ArgumentOutOfRangeException(nameof(index)),
    };

    public IEnumerator<KeyValuePair<string, object?>> GetEnumerator()
    {
        for (var i = 0; i < Count; i++)
        {
            yield return this[i];
        }
    }

    IEnumerator IEnumerable.GetEnumerator() => GetEnumerator();

    public override string ToString() => $"{method} {path} {status} {durationMs}ms";
}
