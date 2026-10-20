using System.Globalization;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Accounts.Api.Serialization;

public readonly record struct Money(decimal Amount, string Currency);

// Writes money as {"amount":"1234.50","currency":"EUR","display":"€1,234.50"}.
public sealed class MoneyConverter(string displayCurrency) : JsonConverter<Money>
{
    public override Money Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options) =>
        throw new NotSupportedException("exports are write-only");

    public override void Write(Utf8JsonWriter writer, Money value, JsonSerializerOptions options)
    {
        var culture = CultureInfo.GetCultures(CultureTypes.SpecificCultures)
            .FirstOrDefault(c => new RegionInfo(c.Name).ISOCurrencySymbol == displayCurrency) ?? CultureInfo.InvariantCulture;
        writer.WriteStartObject();
        writer.WriteString("amount", value.Amount.ToString("F2", CultureInfo.InvariantCulture));
        writer.WriteString("currency", value.Currency);
        writer.WriteString("display", value.Amount.ToString("C", culture));
        writer.WriteEndObject();
    }
}

public sealed class UtcDateTimeConverter : JsonConverter<DateTime>
{
    public override DateTime Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options) =>
        reader.GetDateTime().ToUniversalTime();

    public override void Write(Utf8JsonWriter writer, DateTime value, JsonSerializerOptions options) =>
        writer.WriteStringValue(value.ToUniversalTime().ToString("O", CultureInfo.InvariantCulture));
}
