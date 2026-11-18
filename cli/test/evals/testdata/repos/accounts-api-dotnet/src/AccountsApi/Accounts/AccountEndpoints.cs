using System.Text.Json;
using Accounts.Api.Data;
using Accounts.Api.Serialization;

namespace Accounts.Api.Accounts;

public static class AccountEndpoints
{
    private static readonly JsonSerializerOptions PageOptions = new(JsonSerializerDefaults.Web)
    {
        Converters = { new UtcDateTimeConverter() },
    };

    public static void MapAccountEndpoints(this IEndpointRouteBuilder app)
    {
        app.MapGet("/api/accounts/{accountId}", async (long accountId, AccountRepository accounts, CancellationToken ct) =>
            await accounts.FindAsync(accountId, ct) is { } a ? Results.Ok(a) : Results.NotFound()).RequireAuthorization();

        app.MapGet("/api/accounts/{accountId}/transactions", async (long accountId, int? page, TransactionRepository transactions, CancellationToken ct) =>
        {
            const int pageSize = 50;
            var rows = await transactions.PageAsync(accountId, pageSize, ((page ?? 1) - 1) * pageSize, ct);
            return Results.Json(rows, PageOptions);
        }).RequireAuthorization();
    }
}
