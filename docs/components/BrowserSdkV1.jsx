import Link from "next/link";
import { Callout } from "nextra/components";

export default function BrowserSdkV1({ href }) {
  return (
    <Callout type="info">
      This page documents the 2.x browser SDK. Still on 1.x? See{" "}
      <Link href={href}>the 1.x version of this page</Link>, or follow the{" "}
      <Link href="/client/js-sdk/migration">migration guide</Link> to upgrade.
    </Callout>
  );
}
