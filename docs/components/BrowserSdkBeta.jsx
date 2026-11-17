import Link from "next/link";
import { Callout } from "nextra/components";

export default function BrowserSdkBeta({ stableHref }) {
  return (
    <Callout type="warning">
      <strong>Beta.</strong> The 2.x browser SDK, with remote SDK config
      (OpAMP), the on-device ring buffer and triggers, is in beta. Install it
      with the <code>beta</code> tag:{" "}
      <code>npm install @tracewayapp/frontend@beta</code> (or{" "}
      <code>react@beta</code>, <code>vue@beta</code>, <code>svelte@beta</code>,{" "}
      <code>jquery@beta</code>). A plain <code>npm install</code> still installs
      the stable 1.x SDK, which does not poll for remote settings. Options,
      defaults and the settings document can change before 2.0 is stable.
      {stableHref && (
        <>
          {" "}
          On 1.x? See <Link href={stableHref}>the 1.x version of this page</Link>.
        </>
      )}
    </Callout>
  );
}
