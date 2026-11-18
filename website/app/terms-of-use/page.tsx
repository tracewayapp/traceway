import Link from "next/link";
import type { Metadata } from "next";
import { Eyebrow } from "@/components/eyebrow";
import { LEGAL } from "@/lib/legal";

export const metadata: Metadata = {
  title: "Terms of Use · Traceway",
  description:
    "The terms that govern Traceway Cloud: accounts, plans and billing, your data and our obligations as its processor, acceptable use, liability and termination.",
};

export default function TermsOfUse() {
  return (
    <main className="relative">
      <section className="wrap py-20">
        <article className="prose">
          <Eyebrow>Legal</Eyebrow>
          <h1 className="mt-4 mb-3">Terms of Use</h1>
          <p style={{ color: "var(--fg-3)" }} className="mb-12">
            Last updated: {LEGAL.updated}
          </p>

          <p>
            These terms are a contract between you and {LEGAL.entity} (
            {LEGAL.entityDescription}), {LEGAL.address} (&ldquo;Traceway&rdquo;,
            &ldquo;we&rdquo;, &ldquo;us&rdquo;) for the use of Traceway Cloud,
            the hosted service at cloud.tracewayapp.com (the
            &ldquo;Service&rdquo;). By creating an account or using the
            Service you accept them. If you accept on behalf of a company, you
            confirm that you are authorized to bind it, and &ldquo;you&rdquo;
            means that company. The <Link href="/privacy-policy">Privacy
            Policy</Link> and the <Link href="/dpa">Data Processing
            Addendum</Link> are part of these terms.
          </p>

          <h2>1. Eligibility</h2>
          <p>
            The Service is for businesses, teams and developers. You must be
            at least 18 years old and use the Service for your work or
            projects, not as a consumer.
          </p>

          <h2>2. The Service</h2>
          <p>
            Traceway Cloud is a hosted observability platform. It ingests
            telemetry from your applications through the Traceway SDKs and
            OpenTelemetry (traces, exceptions, logs, metrics, sessions and
            session replays, AI traces, profiles), stores it, and shows it in a
            dashboard together with alerting, on-call paging, synthetic
            monitors, status pages and an API, CLI and MCP server. Features
            marked as beta or preview may change or be withdrawn without
            notice.
          </p>
          <p>
            The same software is available under the MIT license for
            self-hosting. Self-hosted instances are not covered by these terms
            except for section 14, and we provide no warranty or support for
            them unless we agree so in writing.
          </p>

          <h2>3. Your account</h2>
          <p>
            Keep your password, personal access tokens, project tokens and
            source map tokens confidential. You are responsible for everything
            done through your account and tokens, including by AI agents you
            connect through the CLI or MCP server. Tell us at{" "}
            <a href={`mailto:${LEGAL.privacyEmail}`}>{LEGAL.privacyEmail}</a>{" "}
            as soon as you learn of any unauthorized use. You must give us
            accurate account and billing information and keep it current.
          </p>

          <h2>4. Plans, fees and billing</h2>
          <ul>
            <li>
              <strong>Starter</strong> is free and comes with the limits shown
              on the <Link href="/cloud">pricing page</Link>. We may change
              the free plan&apos;s limits with 30 days&apos; notice.
            </li>
            <li>
              <strong>Paid plans</strong> are billed in advance per billing
              period through Stripe and renew automatically until canceled.
              Prices are stated without sales tax, VAT and other taxes, which
              we add where we are required to.
            </li>
            <li>
              <strong>Overages.</strong> If you turn on usage-based overages,
              data ingested above your plan&apos;s allowance is billed at the
              per-gigabyte rate shown in the dashboard at the end of the
              period.
            </li>
            <li>
              <strong>Cancellation.</strong> You can cancel at any time from
              the billing settings. The plan stays active until the end of the
              period you paid for. Fees are not refunded for partial periods,
              except where the law requires it or we have let you down and
              agree to a refund.
            </li>
            <li>
              <strong>Price changes</strong> take effect at your next renewal
              and only after we have emailed you at least 30 days in advance.
            </li>
            <li>
              <strong>Late payment.</strong> If a payment fails, we retry and
              email you. If it still fails after 14 days, we may downgrade or
              suspend the paid features until it is settled.
            </li>
            <li>
              <strong>Enterprise</strong> plans are governed by the order form
              or agreement we sign with you; where it conflicts with these
              terms, the order form wins.
            </li>
          </ul>

          <h2>5. Limits and fair use</h2>
          <p>
            Each plan has limits on exceptions, ingested data, projects,
            members and monitors. The dashboard warns you at 80%. At a limit,
            new data for that meter is paused until the next billing period or
            until you upgrade; nothing already stored is deleted. We may apply
            reasonable rate limits to protect the Service for everyone.
          </p>

          <h2>6. Your data</h2>
          <p>
            Everything you send to the Service, and everything derived from it
            in your dashboard, is <strong>Customer Data</strong>. You own it.
            You give us a license to host, process, display and transmit
            Customer Data only as needed to provide the Service to you, to
            keep it secure, to comply with the law, and as you otherwise
            instruct. We do not sell Customer Data, use it for advertising, or
            use it to train machine-learning models. We may compute aggregate
            statistics that do not identify you or anyone in your data (for
            example total ingest volume) to operate and improve the Service.
          </p>
          <p>
            Where Customer Data contains personal data, you are the
            controller and we are your processor. The{" "}
            <Link href="/dpa">Data Processing Addendum</Link> sets out our
            obligations, including security, sub-processors, breach
            notification and deletion.
          </p>
          <p>You are responsible for Customer Data. In particular, you will:</p>
          <ul>
            <li>
              have a lawful basis for collecting and sending it, and give the
              people it describes the notices the law requires
            </li>
            <li>
              use the SDK masking and sampling options so that you only send
              what you need, and not send payment card numbers, government
              identifiers, health data, passwords or other secrets, or special
              categories of personal data unless they are masked
            </li>
            <li>
              not use the Service to monitor people in a way that is unlawful
              where they live, and not knowingly send us data about children
            </li>
            <li>
              keep your own copies of anything you need to retain beyond the
              retention periods in the Privacy Policy
            </li>
          </ul>

          <h2>7. Acceptable use</h2>
          <p>You will not:</p>
          <ul>
            <li>break the law or infringe anyone&apos;s rights using the Service</li>
            <li>
              probe, scan or attack the Service or other customers&apos;
              projects, or try to access data that is not yours
            </li>
            <li>
              send malicious code, or use synthetic monitors, notifications or
              the API against systems you are not authorized to test or
              contact
            </li>
            <li>
              resell or sublicense the Service, or use it to build a
              competing hosted product using our proprietary parts
            </li>
            <li>
              work around plan limits, billing or rate limits, or create
              multiple free accounts to avoid paying
            </li>
          </ul>
          <p>
            We may remove content or suspend an account that breaks these
            rules. Where practical we warn you first and give you a chance to
            fix it.
          </p>

          <h2>8. Third-party services and integrations</h2>
          <p>
            The Service connects to services you choose, such as Google or
            GitHub for sign-in, Slack, Telegram, Pushover, SMS and webhooks for
            alerts, GitHub for issues, and the AI agents and MCP clients you
            attach. Those services have their own terms, and we are not
            responsible for them. You are responsible for what an agent does
            with the credentials you give it; the agent write log in your
            organization settings is there to help you review it.
          </p>

          <h2>9. Security and availability</h2>
          <p>
            We protect the Service with the measures described in the Privacy
            Policy and the Data Processing Addendum, and we keep improving
            them. We aim for the Service to be available at all times but do
            not promise uninterrupted access on Starter and Premium plans. We
            may run maintenance, and we announce planned maintenance that
            affects availability in advance where we can. Service level
            commitments are available in Enterprise agreements.
          </p>

          <h2>10. Intellectual property</h2>
          <p>
            The Traceway name, logo and branding, the design of the Service,
            and our proprietary infrastructure belong to us. The open-source
            parts of Traceway are licensed under the MIT license published in
            the repository, and nothing here limits that license. If you give
            us feedback or suggestions, we may use them without obligation to
            you.
          </p>

          <h2>11. Confidentiality</h2>
          <p>
            We treat Customer Data and anything you mark as confidential as
            confidential: we use it only to provide the Service, we share it
            only with the sub-processors listed in the Data Processing
            Addendum and with people who need it to help you, and we protect
            it with reasonable care. You will do the same with non-public
            information about the Service that we share with you, such as
            audit reports under NDA.
          </p>

          <h2>12. Warranties and disclaimers</h2>
          <p>
            We warrant that we provide the Service with reasonable skill and
            care and substantially as described on our website. Beyond that,
            and to the extent the law allows, the Service is provided
            &ldquo;as is&rdquo; and we disclaim all other warranties, express
            or implied, including fitness for a particular purpose and
            non-infringement. Monitoring data can be incomplete or delayed;
            you should not rely on the Service as your only safeguard for
            life, safety or regulatory obligations.
          </p>

          <h2>13. Liability</h2>
          <p>
            To the extent the law allows, neither party is liable to the
            other for indirect, incidental, special, consequential or punitive
            damages, or for lost profits, revenue or data, however caused.
            Our total liability under these terms in any 12-month period is
            limited to the fees you paid us in that period, or 100 US dollars
            if you paid none. These limits do not apply to a party&apos;s
            breach of confidentiality, your payment obligations, your
            indemnity under section 14, or liability that cannot be limited by
            law, such as for fraud or for death or personal injury caused by
            negligence.
          </p>

          <h2>14. Indemnity</h2>
          <p>
            You will defend and indemnify us against third-party claims, and
            the resulting damages and reasonable costs, that arise from
            Customer Data, from your breach of section 6 or 7, or from your
            use of a self-hosted instance. We will tell you promptly about any
            such claim and let you control the defense, and we will help you
            at your expense.
          </p>

          <h2>15. Term, termination and deletion</h2>
          <p>
            These terms apply for as long as you have an account. You can
            close your account at any time by cancelling any subscription and
            emailing us. We may suspend or terminate your account on notice if
            you materially breach these terms and do not fix the breach within
            14 days of our notice, if you do not pay, or immediately if your
            use threatens the security or integrity of the Service or we are
            required to by law. We may also discontinue the Service as a whole
            with at least 90 days&apos; notice.
          </p>
          <p>
            Before closure, you can export your data through the dashboard,
            the API and the CLI. After closure, we delete Customer Data and
            your account data within 30 days, except for backups that rotate
            within 35 days and records we must keep by law. Sections 6, 10
            through 14, 15 and 17 survive termination.
          </p>

          <h2>16. Changes to these terms</h2>
          <p>
            We may update these terms. For material changes we email
            organization administrators at least 30 days before the change
            takes effect; other changes take effect when we publish them with a
            new date at the top of this page. If you do not agree to a change,
            you may close your account before it takes effect. Continuing to
            use the Service after that date means you accept it.
          </p>

          <h2>17. Governing law and disputes</h2>
          <p>
            These terms are governed by the laws of {LEGAL.governingLaw},
            without regard to its conflict-of-law rules. Any dispute that we
            cannot settle by talking will be brought before the courts of{" "}
            {LEGAL.venue}, except that mandatory consumer and data protection
            law in your country still applies where it cannot be contracted
            away.
          </p>

          <h2>18. General</h2>
          <p>
            These terms, the Privacy Policy, the Data Processing Addendum and
            any order form are the entire agreement between us about the
            Service. If a part of them is found unenforceable, the rest still
            applies. Neither party is liable for delays caused by events
            outside its reasonable control. You may not assign these terms
            without our consent; we may assign them to a successor of our
            business and will tell you if we do. Notices to you go to the
            email address of your organization&apos;s administrators; notices
            to us go to{" "}
            <a href={`mailto:${LEGAL.privacyEmail}`}>{LEGAL.privacyEmail}</a>.
          </p>

          <h2>19. Contact</h2>
          <p>
            Questions about these terms: {LEGAL.entity}, {LEGAL.address},{" "}
            <a href={`mailto:${LEGAL.privacyEmail}`}>{LEGAL.privacyEmail}</a>, or
            the <Link href="/contact">contact form</Link>.
          </p>
        </article>
      </section>
    </main>
  );
}
