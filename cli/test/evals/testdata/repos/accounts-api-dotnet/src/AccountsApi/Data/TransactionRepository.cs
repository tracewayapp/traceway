using Dapper;
using Npgsql;

namespace Accounts.Api.Data;

public sealed record TransactionRow(
    long Id, DateTime PostedAt, decimal Amount, string Currency, string Counterparty, string? Reference,
    string Category, decimal BalanceAfter, string? Metadata);

public sealed class TransactionRepository(NpgsqlDataSource db)
{
    private const string Columns =
        "t.id, t.posted_at, t.amount, t.currency, t.counterparty, t.reference, t.category, t.balance_after, t.metadata";

    public async Task<List<TransactionRow>> ForPeriodAsync(long accountId, DateTime from, DateTime to, CancellationToken ct)
    {
        await using var conn = await db.OpenConnectionAsync(ct);
        var rows = await conn.QueryAsync<TransactionRow>(new CommandDefinition(
            $"SELECT {Columns} FROM transactions t WHERE t.account_id = @AccountId AND t.posted_at >= @From AND t.posted_at < @To ORDER BY t.posted_at",
            new { AccountId = accountId, From = from, To = to }, cancellationToken: ct));
        return rows.AsList();
    }

    public async Task<List<TransactionRow>> PageAsync(long accountId, int pageSize, int offset, CancellationToken ct)
    {
        await using var conn = await db.OpenConnectionAsync(ct);
        var rows = await conn.QueryAsync<TransactionRow>(new CommandDefinition(
            $"SELECT {Columns} FROM transactions t WHERE t.account_id = @AccountId ORDER BY t.posted_at DESC LIMIT @PageSize OFFSET @Offset",
            new { AccountId = accountId, PageSize = pageSize, Offset = offset }, cancellationToken: ct));
        return rows.AsList();
    }

    public async Task RecordExportAsync(long accountId, string? userId, int rowCount, int byteCount, CancellationToken ct)
    {
        await using var conn = await db.OpenConnectionAsync(ct);
        await conn.ExecuteAsync(new CommandDefinition(
            "INSERT INTO export_audit (account_id, requested_by, row_count, byte_count, created_at) VALUES (@AccountId, @UserId, @RowCount, @ByteCount, now())",
            new { AccountId = accountId, UserId = userId, RowCount = rowCount, ByteCount = byteCount }, cancellationToken: ct));
    }
}

public sealed record AccountRow(long Id, string Number, string Currency, long OwnerId, decimal Balance, DateTime UpdatedAt);

public sealed class AccountRepository(NpgsqlDataSource db)
{
    public async Task<AccountRow?> FindAsync(long accountId, CancellationToken ct)
    {
        await using var conn = await db.OpenConnectionAsync(ct);
        return await conn.QuerySingleOrDefaultAsync<AccountRow>(new CommandDefinition(
            "SELECT id, number, currency, owner_id, balance, updated_at FROM accounts WHERE id = @AccountId",
            new { AccountId = accountId }, cancellationToken: ct));
    }
}
