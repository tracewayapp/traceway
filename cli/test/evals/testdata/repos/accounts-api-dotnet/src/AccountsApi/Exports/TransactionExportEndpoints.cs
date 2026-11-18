using System.Security.Claims;
using System.Text;
using System.Text.Json;
using Accounts.Api.Data;
using Accounts.Api.Serialization;

namespace Accounts.Api.Exports;

public static class TransactionExportEndpoints
{
    public static void MapTransactionExportEndpoints(this IEndpointRouteBuilder app)
    {
        app.MapGet("/api/accounts/{accountId}/transactions/export", ExportAsync).RequireAuthorization();
    }

    // Finance downloads a period's transactions for reconciliation; the
    // dashboard saves the body as <account>-<from>-<to>.json.
    private static async Task<IResult> ExportAsync(
        long accountId, DateTime from, DateTime to, ClaimsPrincipal user,
        AccountRepository accounts, TransactionRepository transactions, ILogger<TransactionExportEndpointsLog> logger,
        CancellationToken ct)
    {
        var account = await accounts.FindAsync(accountId, ct);
        if (account is null)
        {
            return Results.NotFound();
        }
        if (to <= from)
        {
            return Results.UnprocessableEntity(new { error = "to must be after from" });
        }

        var rows = await transactions.ForPeriodAsync(accountId, from, to, ct);

        var body = new StringBuilder();
        body.Append("{\"account\":").Append(JsonSerializer.Serialize(account.Number))
            .Append(",\"currency\":").Append(JsonSerializer.Serialize(account.Currency))
            .Append(",\"transactions\":[");
        for (var i = 0; i < rows.Count; i++)
        {
            if (i > 0) body.Append(',');
            body.Append(SerializeRow(rows[i], account.Currency));
        }
        body.Append("]}");
        var json = body.ToString();
        var bytes = Encoding.UTF8.GetByteCount(json);

        await transactions.RecordExportAsync(accountId, user.FindFirstValue(ClaimTypes.NameIdentifier), rows.Count, bytes, ct);
        logger.LogInformation("Exported {RowCount} transactions ({ByteCount} bytes) for account {AccountId}", rows.Count, bytes, accountId);

        return Results.Text(json, "application/json", Encoding.UTF8);
    }

    private static string SerializeRow(TransactionRow row, string currency)
    {
        var options = new JsonSerializerOptions(JsonSerializerDefaults.Web)
        {
            WriteIndented = true,
            Converters = { new MoneyConverter(currency), new UtcDateTimeConverter() },
        };
        var export = new TransactionExport(
            row.Id,
            row.PostedAt,
            new Money(row.Amount, row.Currency),
            row.Counterparty,
            row.Reference,
            row.Category,
            new Money(row.BalanceAfter, row.Currency),
            JsonDocument.Parse(row.Metadata ?? "{}").RootElement.Clone());
        return JsonSerializer.Serialize(export, options);
    }
}

public sealed class TransactionExportEndpointsLog;

public sealed record TransactionExport(
    long Id, DateTime PostedAt, Money Amount, string Counterparty, string? Reference,
    string Category, Money BalanceAfter, JsonElement Metadata);
