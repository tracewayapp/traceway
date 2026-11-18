import Link from "next/link";
import type { Metadata } from "next";
import { Eyebrow } from "@/components/eyebrow";
import { SubprocessorTable } from "@/components/subprocessor-table";
import { LEGAL } from "@/lib/legal";

export const metadata: Metadata = {
  title: "Data Processing Addendum · Traceway",
  description:
    "The GDPR Article 28 data processing addendum for Traceway Cloud: processing details, security measures, sub-processors, breach notification, deletion and international transfers.",
};

export default function DataProcessingAddendum() {
  return (
    <main className="relative">
      <section className="wrap py-20">
        <article className="prose">
          <Eyebrow>Legal</Eyebrow>
          <h1 className="mt-4 mb-3">Data Processing Addendum</h1>
          <p style={{ color: "var(--fg-3)" }} className="mb-12">
            Last updated: {LEGAL.updated}
          </p>

          <div className="legal-note">
            This addendum (&ldquo;DPA&rdquo;) is part of the{" "}
            <Link href="/terms-of-use">Terms of Use</Link> and applies
            automatically to every customer of Traceway Cloud whose use of the
            Service involves personal data protected by the GDPR, the UK GDPR,
            the Swiss FADP or a similar law. No signature is needed. If your
            procurement process requires a countersigned copy, email{" "}
            <a href={`mailto:${LEGAL.privacyEmail}`}>{LEGAL.privacyEmail}</a>{" "}
            and we will send one.
          </div>

          <h2>1. Parties and roles</h2>
          <p>
            This DPA is between the customer that accepted the Terms of Use
            (&ldquo;Customer&rdquo;) and {LEGAL.entity} ({LEGAL.entityDescription}),{" "}
            {LEGAL.address} (&ldquo;Traceway&rdquo;). For the personal data
            contained in Customer Data (defined in the Terms), Customer is the
            controller, or a processor acting for another controller, and
            Traceway is Customer&apos;s processor. For the account, billing and
            security data of Customer&apos;s users, Traceway is an independent
            controller, as described in the{" "}
            <Link href="/privacy-policy#cloud-accounts">Privacy Policy</Link>,
            and that data is not covered by this DPA.
          </p>

          <h2>2. Definitions</h2>
          <p>
            &ldquo;Data Protection Law&rdquo; means every law that applies to
            the processing of personal data under this DPA, including the GDPR
            (Regulation (EU) 2016/679), the UK GDPR and Data Protection Act
            2018, and the Swiss Federal Act on Data Protection.
            &ldquo;Personal data&rdquo;, &ldquo;controller&rdquo;,
            &ldquo;processor&rdquo;, &ldquo;data subject&rdquo;,
            &ldquo;processing&rdquo; and &ldquo;personal data breach&rdquo;
            have the meanings given in the GDPR. &ldquo;Customer Personal
            Data&rdquo; means the personal data in Customer Data.
            &ldquo;Sub-processor&rdquo; means a third party Traceway engages to
            process Customer Personal Data. &ldquo;SCCs&rdquo; means the
            standard contractual clauses adopted by the European Commission in
            Decision (EU) 2021/914.
          </p>

          <h2>3. Details of the processing</h2>
          <p>
            The subject matter, duration, nature and purpose of the
            processing, and the types of personal data and categories of data
            subjects, are described in Annex 1.
          </p>

          <h2>4. Customer&apos;s obligations</h2>
          <p>
            Customer is responsible for the lawfulness of the Customer
            Personal Data it sends to the Service, for having a lawful basis,
            for giving data subjects the required information, and for
            configuring the SDKs and exporters (sampling, masking, attribute
            allow-lists, privacy mode for AI traces) so that only the personal
            data it needs is sent. Customer will not send special categories
            of personal data, payment card data or data about children unless
            it is masked, and will tell Traceway if it is legally required to
            apply stricter rules.
          </p>

          <h2>5. Traceway&apos;s obligations</h2>
          <p>Traceway will:</p>
          <ul>
            <li>
              process Customer Personal Data only on Customer&apos;s documented
              instructions, which are the Terms, this DPA, and the settings
              Customer chooses in the Service. Traceway will tell Customer if
              it believes an instruction infringes Data Protection Law, and if
              a law requires Traceway to process the data otherwise, it will
              tell Customer before doing so unless that law forbids it
            </li>
            <li>
              make sure everyone who can access Customer Personal Data is
              bound by confidentiality and has access only to the extent
              needed to provide, secure and support the Service
            </li>
            <li>
              implement and maintain the technical and organizational measures
              in Annex 2, and not reduce their overall level of protection
              during the term
            </li>
            <li>
              engage Sub-processors only under section 6
            </li>
            <li>
              help Customer respond to data subject requests (section 8), and
              to carry out data protection impact assessments and prior
              consultations, taking into account the nature of the processing
              and the information available to Traceway
            </li>
            <li>
              notify personal data breaches under section 9
            </li>
            <li>
              delete or return Customer Personal Data under section 10
            </li>
            <li>
              provide the information needed to demonstrate compliance and
              allow audits under section 11
            </li>
          </ul>

          <h2 id="subprocessors">6. Sub-processors</h2>
          <p>
            Customer gives Traceway general authorization to use the
            Sub-processors listed in Annex 3. Traceway imposes data protection
            obligations on each Sub-processor that are no less protective than
            those in this DPA, and remains fully liable to Customer for their
            performance. Traceway will give Customer at least 30 days&apos;
            notice by email to organization administrators before adding or
            replacing a Sub-processor that processes Customer Personal Data.
            Customer may object on reasonable data protection grounds within
            that period. If the parties cannot resolve the objection in good
            faith within 30 days, Customer may terminate the affected
            subscription and Traceway refunds any prepaid fees for the
            remaining term.
          </p>

          <h2 id="transfers">7. International transfers</h2>
          <p>
            Traceway stores and processes Customer Personal Data in the United
            States (Annex 3). Where Data Protection Law restricts a transfer
            of Customer Personal Data from the EEA, the United Kingdom or
            Switzerland to Traceway or a Sub-processor, the parties rely on,
            in this order: an adequacy decision that covers the recipient
            (including the EU-US Data Privacy Framework where the recipient is
            certified), and otherwise the SCCs, which are incorporated into
            this DPA by reference as follows:
          </p>
          <ul>
            <li>
              Module Two (controller to processor) applies where Customer is a
              controller, and Module Three (processor to processor) where
              Customer is a processor
            </li>
            <li>
              Clause 7 (docking) is included; Clause 9 option 2 (general
              authorization) applies with the notice period in section 6;
              Clause 11 optional language does not apply; Clause 13 is
              completed by the supervisory authority of Customer&apos;s EU
              establishment, or otherwise of its EU representative; Clause 17
              option 1 and Clause 18 apply with the law and courts of Ireland
            </li>
            <li>
              Annex I and Annex II of the SCCs are completed by Annex 1, Annex
              2 and Annex 3 of this DPA
            </li>
            <li>
              for transfers from the United Kingdom, the UK International Data
              Transfer Addendum to the SCCs issued by the Information
              Commissioner applies, with the tables completed by the same
              Annexes; for transfers from Switzerland, the SCCs are adapted as
              required by the Swiss Federal Data Protection and Information
              Commissioner
            </li>
          </ul>
          <p>
            Traceway will tell Customer if it can no longer comply with the
            transfer mechanism relied on, and Customer may then suspend the
            transfer and terminate the affected subscription.
          </p>

          <h2>8. Data subject requests</h2>
          <p>
            The Service gives Customer the tools to search, view and delete
            telemetry, sessions, replays and projects, which is how most data
            subject requests can be honored directly. If Traceway receives a
            request from a data subject concerning Customer Personal Data, it
            will not answer it on the merits but will forward it to Customer
            without undue delay, where Traceway can identify Customer. Where
            Customer cannot fulfil a request with the tools provided, Traceway
            will help on request, at no charge for reasonable effort.
          </p>

          <h2>9. Personal data breaches</h2>
          <p>
            Traceway will notify Customer without undue delay, and in any
            event within 72 hours, after becoming aware of a personal data
            breach affecting Customer Personal Data. The notice goes by email
            to organization administrators and describes, to the extent known,
            the nature of the breach, the categories and approximate number of
            data subjects and records concerned, the likely consequences, the
            measures taken or proposed, and a contact point. Traceway will
            update the notice as more becomes known and will cooperate with
            Customer&apos;s own notification obligations.
          </p>

          <h2>10. Deletion and return</h2>
          <p>
            Customer can delete projects and individual data from the Service
            at any time, and export Customer Data through the dashboard, API
            and CLI. Within 30 days after the end of the subscription or the
            closure of the account, Traceway deletes all Customer Personal
            Data, except for copies in backups, which are overwritten on their
            rotation schedule within a further 35 days, and data Traceway must
            keep under a legal obligation, which it continues to protect under
            this DPA. On written request Traceway confirms deletion.
          </p>

          <h2>11. Audits</h2>
          <p>
            On request, no more than once a year unless a breach or a
            supervisory authority requires otherwise, Traceway will make
            available the information reasonably necessary to demonstrate
            compliance with this DPA, including its security documentation and,
            once available, its SOC 2 Type II and ISO 27001 reports under NDA.
            Where that information is not sufficient, Customer or an
            independent auditor it mandates may audit Traceway&apos;s relevant
            controls, on at least 30 days&apos; notice, during business hours,
            without disrupting the Service, at Customer&apos;s cost, and
            subject to confidentiality. The parties agree the scope in advance.
          </p>

          <h2>12. Liability and term</h2>
          <p>
            Each party&apos;s liability under this DPA is subject to the
            limitations in the Terms of Use, except where Data Protection Law
            does not allow a limitation. This DPA applies for as long as
            Traceway processes Customer Personal Data. If this DPA conflicts
            with the Terms, this DPA wins for the processing of Customer
            Personal Data; if it conflicts with the SCCs, the SCCs win.
          </p>

          <h2>Annex 1: Processing details</h2>
          <div className="table-scroll">
            <table>
              <tbody>
                <tr>
                  <th>Subject matter</th>
                  <td>
                    Hosting and processing of application telemetry so that
                    Customer can monitor, debug and operate its software
                  </td>
                </tr>
                <tr>
                  <th>Duration</th>
                  <td>
                    The term of the subscription plus the deletion period in
                    section 10
                  </td>
                </tr>
                <tr>
                  <th>Nature and purpose</th>
                  <td>
                    Receiving telemetry over HTTPS, storing it, indexing and
                    aggregating it, displaying it in the dashboard, evaluating
                    alert rules and delivering notifications, replaying
                    sessions, symbolicating stack traces, and answering API,
                    CLI and MCP queries, all on Customer&apos;s behalf
                  </td>
                </tr>
                <tr>
                  <th>Categories of data subjects</th>
                  <td>
                    End users of Customer&apos;s applications; Customer&apos;s
                    employees and contractors whose activity appears in logs,
                    traces and AI conversations
                  </td>
                </tr>
                <tr>
                  <th>Types of personal data</th>
                  <td>
                    Whatever Customer chooses to send, typically: IP addresses
                    of devices that report sessions, user identifiers and
                    attributes attached to sessions and spans (which may
                    include names or email addresses), device and browser
                    information, URLs and request parameters, log and
                    exception messages, session replay recordings of the
                    application screen with Customer&apos;s masking applied,
                    prompts and completions in AI traces unless privacy mode
                    is enabled
                  </td>
                </tr>
                <tr>
                  <th>Special categories</th>
                  <td>
                    Not intended. Customer must mask them if its application
                    handles them
                  </td>
                </tr>
                <tr>
                  <th>Frequency</th>
                  <td>Continuous, as telemetry is sent</td>
                </tr>
                <tr>
                  <th>Retention</th>
                  <td>
                    As set out in the{" "}
                    <Link href="/privacy-policy#retention">Privacy Policy</Link>
                    , and within 30 days of project deletion or account closure
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <h2>Annex 2: Technical and organizational measures</h2>
          <ul>
            <li>
              <strong>Encryption.</strong> TLS 1.2 or higher for every
              connection to the Service and between the Service and its
              Sub-processors. Encryption at rest with AWS-managed keys for
              databases, object storage and backups.
            </li>
            <li>
              <strong>Credentials.</strong> Passwords stored as salted hashes.
              API tokens, personal access tokens, refresh tokens and SMS
              verification codes stored only as SHA-256 hashes. Short-lived
              access tokens with rotating refresh tokens and family revocation
              on reuse. Write access for AI agents requires an explicit
              organization opt-in and is logged.
            </li>
            <li>
              <strong>Tenant isolation.</strong> Every row of telemetry is
              keyed by project, every query is scoped by the caller&apos;s
              organization membership, and role checks (owner, admin, user,
              read-only) are enforced server side. Synthetic monitors on Cloud
              cannot reach private networks.
            </li>
            <li>
              <strong>Access control.</strong> Production access is limited to
              the founders, requires SSH keys and multi-factor authentication,
              and is used only to operate the Service and for support at
              Customer&apos;s request. No one browses Customer Data for
              curiosity.
            </li>
            <li>
              <strong>Availability.</strong> Infrastructure on AWS with
              automated restarts, database backups rotated within 35 days,
              rate limits and admission control on ingest so overload sheds
              load instead of losing the process.
            </li>
            <li>
              <strong>Software security.</strong> Open-source code with public
              review, dependency vulnerability scanning in CI, signed
              releases, and a published{" "}
              <a
                href="https://github.com/tracewayapp/traceway/blob/main/SECURITY.md"
                target="_blank"
                rel="noopener noreferrer"
              >
                security policy
              </a>{" "}
              for reporting vulnerabilities.
            </li>
            <li>
              <strong>Data minimization tools.</strong> SDK-side masking of
              inputs and text in session replays, privacy masks in mobile
              replays, attribute allow-lists for metrics, privacy mode for AI
              traces, and configurable retention and deletion.
            </li>
            <li>
              <strong>Organization.</strong> Confidentiality obligations for
              everyone with access, incident response with the notification
              commitments in section 9, and SOC 2 Type II and ISO 27001
              certification in progress.
            </li>
          </ul>

          <h2 id="annex-3">Annex 3: Sub-processors</h2>
          <p>
            The following Sub-processors process Customer Personal Data. The
            list is also published in the{" "}
            <Link href="/privacy-policy#sharing">Privacy Policy</Link>, where
            you can see every provider we use, including those that never
            touch Customer Data.
          </p>
          <SubprocessorTable customerDataOnly />
          <p>
            Notification destinations that Customer configures (Slack,
            Telegram, Pushover, GitHub, webhooks) are not Sub-processors:
            Customer chooses them and they receive data under Customer&apos;s
            own agreements with them.
          </p>
        </article>
      </section>
    </main>
  );
}
