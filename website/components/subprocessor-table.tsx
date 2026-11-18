import { SUBPROCESSORS } from "@/lib/legal";

export function SubprocessorTable({
  customerDataOnly = false,
}: {
  customerDataOnly?: boolean;
}) {
  const rows = customerDataOnly
    ? SUBPROCESSORS.filter((s) => s.customerData)
    : SUBPROCESSORS;
  return (
    <div className="table-scroll">
      <table>
        <thead>
          <tr>
            <th>Provider</th>
            <th>Location</th>
            <th>What for</th>
            {customerDataOnly ? null : <th>Customer telemetry</th>}
            <th>Transfer safeguard</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((s) => (
            <tr key={s.name}>
              <td>
                <strong>{s.name}</strong>
                <br />
                <span style={{ color: "var(--fg-2)" }}>{s.entity}</span>
              </td>
              <td>{s.location}</td>
              <td>{s.purpose}</td>
              {customerDataOnly ? null : <td>{s.customerData ? "Yes" : "No"}</td>}
              <td>{s.transfer}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
