using Accounts.Api;
using Accounts.Api.Accounts;
using Accounts.Api.Data;
using Accounts.Api.Exports;
using Dapper;
using Npgsql;
using OpenTelemetry.Logs;
using OpenTelemetry.Resources;
using OpenTelemetry.Trace;

var builder = WebApplication.CreateBuilder(args);

DefaultTypeMap.MatchNamesWithUnderscores = true;
builder.Services.AddSingleton(_ => NpgsqlDataSource.Create(builder.Configuration.GetConnectionString("Ledger")!));
builder.Services.AddScoped<TransactionRepository>();
builder.Services.AddScoped<AccountRepository>();

builder.Services.AddOpenTelemetry()
    .ConfigureResource(r => r.AddService("accounts-api"))
    .WithTracing(t => t
        .AddAspNetCoreInstrumentation()
        .AddNpgsql()
        .AddOtlpExporter());
builder.Logging.AddOpenTelemetry(o =>
{
    o.IncludeFormattedMessage = true;
    o.AddOtlpExporter();
});

builder.Services.AddAuthentication().AddJwtBearer();
builder.Services.AddAuthorization();

var app = builder.Build();
app.UseAccessLog();
app.UseAuthentication();
app.UseAuthorization();

app.MapAccountEndpoints();
app.MapTransactionExportEndpoints();

app.Run();
