import Link from "next/link";
import type { Metadata } from "next";
import { Eyebrow } from "@/components/eyebrow";
import { SubprocessorTable } from "@/components/subprocessor-table";
import { CookieSettingsLink } from "@/components/cookie-settings-link";
import { LEGAL } from "@/lib/legal";

export const metadata: Metadata = {
  title: "Privacy Policy · Traceway",
  description:
    "What Traceway collects on tracewayapp.com and in Traceway Cloud, why, how long it is kept, who it is shared with, and the rights you have under the GDPR.",
};

export default function PrivacyPolicy() {
  return (
    <main className="relative">
      <section className="wrap py-20">
        <article className="prose">
          <Eyebrow>Legal</Eyebrow>
          <h1 className="mt-4 mb-3">Privacy Policy</h1>
          <p style={{ color: "var(--fg-3)" }} className="mb-12">
            Last updated: {LEGAL.updated}
          </p>

          <p>
            This policy explains what personal data we collect when you visit
            our websites or use Traceway Cloud, why we collect it, how long we
            keep it, who we share it with, and what rights you have. We wrote
            it to be read, so it is specific about the data and the systems
            involved. If anything is unclear, write to{" "}
            <a href={`mailto:${LEGAL.privacyEmail}`}>{LEGAL.privacyEmail}</a>.
          </p>

          <h2 id="who-we-are">1. Who we are</h2>
          <p>
            The data controller is {LEGAL.entity} ({LEGAL.entityDescription}),{" "}
            {LEGAL.address} (&ldquo;Traceway&rdquo;, &ldquo;we&rdquo;,
            &ldquo;us&rdquo;). We operate the website tracewayapp.com, the
            documentation site docs.tracewayapp.com, and the hosted service
            Traceway Cloud at cloud.tracewayapp.com. We have not appointed a
            data protection officer because the law does not require one for
            an organization of our size; the contact above reaches the people
            responsible for privacy directly.
          </p>

          <h2 id="scope">2. Three situations this policy covers</h2>
          <p>
            Depending on how you interact with us, we play one of two roles
            under the GDPR:
          </p>
          <ul>
            <li>
              <strong>Website visitors and people who contact us.</strong> We
              are the controller of the little data this involves. Section 3.
            </li>
            <li>
              <strong>Traceway Cloud account holders.</strong> We are the
              controller of your account, billing and security data. Section
              4.
            </li>
            <li>
              <strong>
                People whose data appears in the telemetry our customers send.
              </strong>{" "}
              If a company uses Traceway Cloud to monitor its application, the
              telemetry can contain data about that company&apos;s own users.
              The company is the controller of that data and we are its
              processor, bound by our{" "}
              <Link href="/dpa">Data Processing Addendum</Link>. Section 5
              describes that data; section 11 explains how to exercise your
              rights in that case.
            </li>
          </ul>
          <p>
            Traceway is also open-source software you can run yourself. A
            self-hosted instance sends nothing to us. Section 14.
          </p>

          <h2 id="website">3. The websites</h2>
          <h3>Connection data</h3>
          <p>
            tracewayapp.com and docs.tracewayapp.com are static sites served
            through Cloudflare. To deliver pages and block abuse, Cloudflare
            processes the connection data every web server sees: your IP
            address, user agent, the requested URL and timing. We only see
            aggregate statistics from it. Legal basis: our legitimate interest
            in running a secure website (Art. 6(1)(f) GDPR).
          </p>
          <p>
            The contact and subscribe forms are handled by a small Cloudflare
            Worker. Its performance traces (URL, method, status and duration,
            never the form contents) are recorded in our own Traceway Cloud
            instance and kept for up to 90 days.
          </p>
          <h3>Analytics, only with your consent</h3>
          <p>
            We use Google Analytics 4 to understand which pages are read and
            where visitors come from. If you visit from the EEA, the United
            Kingdom or Switzerland, it is off until you choose &ldquo;Accept
            all&rdquo; or enable analytics in the cookie banner. Elsewhere it
            runs by default and you can turn it off, and you can change your
            mind at any time either way via{" "}
            <CookieSettingsLink className="underline underline-offset-4 cursor-pointer" />{" "}
            in the footer. Google Analytics sets the <code>_ga</code> cookies
            listed in section 6, truncates IP addresses, and we have
            advertising features and Google Signals turned off. Google LLC
            processes this data in the United States under the EU-US Data
            Privacy Framework and keeps it for at most 14 months. Legal basis
            where the GDPR applies: consent (Art. 6(1)(a)), which you can
            withdraw at any time with effect for the future. We decide which
            rule applies from the country your connection comes from, which
            Cloudflare tells us; we do not store it.
          </p>
          <h3>The contact form</h3>
          <p>
            When you submit the form on <Link href="/contact">/contact</Link>,
            your first name, last name, email address and company size are
            emailed to us through Twilio SendGrid and land in our mailbox. We
            use them to reply to you and to set up your account or a call. If
            you then book a call, you are taken to Calendly with your name and
            email pre-filled; Calendly&apos;s own privacy policy applies from
            that point. We keep contact submissions for 12 months after our
            last exchange, or for as long as we are in a business relationship
            with you. Legal basis: steps you ask us to take before a contract
            (Art. 6(1)(b)) and our legitimate interest in answering enquiries
            (Art. 6(1)(f)).
          </p>
          <h3>The blog subscription</h3>
          <p>
            If you subscribe on the blog, we keep your email address and send
            you an email when we publish a new engineering post. We send these
            ourselves; there is no newsletter platform behind it. You can
            unsubscribe at any time by replying to any post email or writing
            to {LEGAL.privacyEmail}, and we delete your address when you do.
            Legal basis: consent (Art. 6(1)(a)).
          </p>
          <h3>Links to other services</h3>
          <p>
            The site links to GitHub, Discord and Calendly. Those are separate
            services with their own privacy policies. We do not embed their
            trackers in our pages; fonts are bundled with the site rather than
            loaded from Google, and videos are served from our own hosting.
          </p>

          <h2 id="cloud-accounts">4. Traceway Cloud accounts</h2>
          <p>
            This section covers the data we hold about you as a person who
            uses Traceway Cloud. The telemetry your applications send is
            covered in section 5.
          </p>
          <h3>What we collect and why</h3>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Data</th>
                  <th>Why we need it</th>
                  <th>Legal basis</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>
                    Name, email address, password (stored only as a hash),
                    timezone, organization name, your role in each organization
                  </td>
                  <td>
                    To create and secure your account and show you the right
                    projects
                  </td>
                  <td>Contract, Art. 6(1)(b)</td>
                </tr>
                <tr>
                  <td>
                    Google or GitHub account id, name, email and avatar URL
                    when you sign in with one of them
                  </td>
                  <td>To sign you in without a password</td>
                  <td>Contract, Art. 6(1)(b)</td>
                </tr>
                <tr>
                  <td>Email addresses of people you invite</td>
                  <td>To deliver the invitation and attach them to your organization</td>
                  <td>Legitimate interest of you and the invitee, Art. 6(1)(f)</td>
                </tr>
                <tr>
                  <td>
                    On-call contact methods you add: phone number (verified
                    with a code sent by SMS), Slack, Telegram or Pushover
                    identifiers, email
                  </td>
                  <td>To page you when an alert escalates to you</td>
                  <td>Contract, Art. 6(1)(b); you add and remove these yourself</td>
                </tr>
                <tr>
                  <td>
                    Login sessions, personal access tokens, CLI and MCP tokens
                    (all stored as hashes), device authorizations, and an audit
                    log of writes made by AI agents with your credentials
                  </td>
                  <td>To keep your account secure and let you review agent activity</td>
                  <td>Contract, Art. 6(1)(b) and legitimate interest in security, Art. 6(1)(f)</td>
                </tr>
                <tr>
                  <td>
                    Stripe customer id, plan, subscription status, daily usage
                    counters (exceptions and gigabytes ingested per
                    organization), invoices
                  </td>
                  <td>To bill you and enforce plan limits</td>
                  <td>Contract, Art. 6(1)(b); legal obligation for invoices, Art. 6(1)(c)</td>
                </tr>
                <tr>
                  <td>
                    Emails we send you: invitations, password resets, alert and
                    on-call notifications, billing and service announcements
                  </td>
                  <td>To operate the service you signed up for</td>
                  <td>Contract, Art. 6(1)(b)</td>
                </tr>
                <tr>
                  <td>Support conversations by email or in a shared Slack channel</td>
                  <td>To help you</td>
                  <td>Contract, Art. 6(1)(b)</td>
                </tr>
                <tr>
                  <td>
                    Errors and performance data of Traceway Cloud itself,
                    recorded in a Traceway project of our own, which can
                    include request URLs, timing and your account id
                  </td>
                  <td>To keep the service running and fix bugs</td>
                  <td>Legitimate interest, Art. 6(1)(f)</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p>
            Card details are entered on pages hosted by Stripe and never reach
            our servers. The sign-up form uses Cloudflare Turnstile to keep
            bots out; Turnstile analyzes browser signals during the check and
            does not track you across sites. IP addresses are used in memory
            for rate limiting and are not stored with your account.
          </p>
          <p>
            We do not send marketing email to account holders without a
            separate opt-in, we do not sell personal data, and we do not use
            your data for advertising.
          </p>

          <h2 id="telemetry">5. Telemetry your applications send</h2>
          <p>
            When you connect an application to Traceway Cloud, your SDKs and
            OpenTelemetry exporters send us telemetry. You decide what is in
            it. It can include:
          </p>
          <ul>
            <li>
              Traces and spans with their attributes, including HTTP routes,
              status codes, database statements and any attribute you attach
            </li>
            <li>Exceptions with stack traces, messages and tags</li>
            <li>Log records with their bodies and attributes</li>
            <li>Metrics with their tags</li>
            <li>
              Browser and mobile sessions: a session id, start and end time,
              app version, the IP address of the device that sent the session
              (recorded by our server), and attributes your code attaches, for
              example a user id or email
            </li>
            <li>
              Session replays: DOM recordings from the browser SDK, or
              screenshots-based video from the Flutter SDK, with the masking
              rules you configure applied on the device before upload
            </li>
            <li>
              AI traces: model, token counts and cost, and, unless you enable
              privacy mode in your exporter, the prompts and completions
              themselves, which are scanned at ingest for terms you configure
            </li>
            <li>Profiles, synthetic monitor results, source maps and debug symbols</li>
          </ul>
          <p>
            For this data you are the controller and we are your processor. We
            process it only to provide the service to you, as set out in the{" "}
            <Link href="/dpa">Data Processing Addendum</Link>, and we never use
            it for our own purposes, to train models, or to profile the people
            it describes. Our SDK documentation explains how to mask inputs,
            exclude fields and avoid sending data you do not need.
          </p>

          <h2 id="cookies">6. Cookies and local storage</h2>
          <p>
            We use as few cookies as we can. Nothing on our sites is used for
            advertising or cross-site tracking.
          </p>
          <h3>tracewayapp.com</h3>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Set by</th>
                  <th>Purpose</th>
                  <th>Type</th>
                  <th>Lifetime</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td><code>tw_consent</code></td>
                  <td>Traceway</td>
                  <td>Remembers the choice you made in the cookie banner</td>
                  <td>Strictly necessary</td>
                  <td>6 months</td>
                </tr>
                <tr>
                  <td><code>__cf_bm</code>, <code>cf_clearance</code></td>
                  <td>Cloudflare</td>
                  <td>
                    Bot protection, set only when Cloudflare decides a check is
                    needed
                  </td>
                  <td>Strictly necessary</td>
                  <td>Up to 30 minutes, or the duration of a passed challenge</td>
                </tr>
                <tr>
                  <td><code>_ga</code></td>
                  <td>Google Analytics</td>
                  <td>Tells returning visitors apart in aggregate statistics</td>
                  <td>Analytics, opt-in in the EEA, UK and Switzerland, opt-out elsewhere</td>
                  <td>2 years</td>
                </tr>
                <tr>
                  <td><code>_ga_KSB465GF2W</code></td>
                  <td>Google Analytics</td>
                  <td>Keeps the state of your current visit</td>
                  <td>Analytics, opt-in in the EEA, UK and Switzerland, opt-out elsewhere</td>
                  <td>2 years</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p>
            If you reject analytics or opt out later, we turn the Google
            Analytics tag off and delete its cookies from your browser. We
            honor the Global Privacy Control signal everywhere: when your
            browser sends it, analytics stays off unless you turn it on
            yourself.
          </p>
          <h3>docs.tracewayapp.com</h3>
          <p>
            The documentation site sets no cookies of its own and loads no
            analytics. Cloudflare may set the security cookies listed above.
          </p>
          <h3>cloud.tracewayapp.com</h3>
          <p>
            The dashboard is a web application and needs to remember who you
            are. Everything it stores is strictly necessary and stays in your
            browser:
          </p>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Where</th>
                  <th>Purpose</th>
                  <th>Lifetime</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td><code>AUTH_TOKEN</code></td>
                  <td>Local storage</td>
                  <td>Your login session</td>
                  <td>Valid for 7 days, removed on logout</td>
                </tr>
                <tr>
                  <td>
                    <code>CURRENT_PROJECT_ID</code>,{" "}
                    <code>USER_ORGANIZATIONS</code>,{" "}
                    <code>PROJECTS_CACHE_V2</code>
                  </td>
                  <td>Local storage</td>
                  <td>Which project and organization you were looking at</td>
                  <td>Until logout</td>
                </tr>
                <tr>
                  <td>
                    <code>traceway_theme</code>, <code>traceway_sort_*</code>{" "}
                    and time-range preferences
                  </td>
                  <td>Local storage</td>
                  <td>Dark or light mode, table sorting, selected time ranges</td>
                  <td>Until you clear your browser data</td>
                </tr>
                <tr>
                  <td><code>sidebar:state</code></td>
                  <td>Cookie</td>
                  <td>Whether you collapsed the sidebar</td>
                  <td>7 days</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p>
            The sign-up page loads Cloudflare Turnstile, which may set a
            Cloudflare cookie for the duration of the bot check. The dashboard
            loads no analytics.
          </p>

          <h2 id="sharing">7. Who we share data with</h2>
          <p>
            We share personal data only with the service providers below, who
            act on our instructions under data processing agreements, and
            otherwise only where the law requires it. The column
            &ldquo;Customer telemetry&rdquo; marks the providers that can come
            into contact with the data described in section 5; they are also
            the sub-processors listed in our{" "}
            <Link href="/dpa#subprocessors">Data Processing Addendum</Link>.
          </p>
          <SubprocessorTable />
          <p>
            Some services receive data because you chose to use them, and they
            act as independent controllers: Google and GitHub when you sign in
            with them, Calendly when you book a call, and the destinations you
            configure for alerts and paging (Slack, Telegram, Pushover,
            GitHub issues, webhooks). We will disclose data to authorities only
            when we are legally required to, and we will tell you first unless
            the law forbids it.
          </p>

          <h2 id="transfers">8. Where your data is stored</h2>
          <p>
            We are a United States company, and Traceway Cloud runs on Amazon
            Web Services in the United States (us-east-1, N. Virginia). The
            websites are served from
            Cloudflare&apos;s global network. Our email and SMS providers are
            in the United States. If you are in the EEA, the United Kingdom or
            Switzerland, this means your data is transferred to a country
            without a general adequacy decision. We rely on the EU-US Data
            Privacy Framework for providers certified under it and on the
            European Commission&apos;s Standard Contractual Clauses for the
            rest, together with the technical measures in section 10. The
            Standard Contractual Clauses between you and us are part of the{" "}
            <Link href="/dpa#transfers">Data Processing Addendum</Link>.
          </p>

          <h2 id="retention">9. How long we keep data</h2>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Data</th>
                  <th>Kept for</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>Account and organization data</td>
                  <td>
                    While your account exists, then deleted within 30 days of
                    closure
                  </td>
                </tr>
                <tr>
                  <td>Invoices and billing records</td>
                  <td>As long as accounting and tax law requires, up to 10 years</td>
                </tr>
                <tr>
                  <td>Expired login sessions, tokens and device authorizations</td>
                  <td>Pruned daily; used refresh tokens kept 30 days for replay detection</td>
                </tr>
                <tr>
                  <td>Agent write audit log</td>
                  <td>90 days</td>
                </tr>
                <tr>
                  <td>Log records</td>
                  <td>90 days</td>
                </tr>
                <tr>
                  <td>Metrics</td>
                  <td>
                    Raw points 7 days, one-minute rollups 30 days, hourly
                    rollups 1 year
                  </td>
                </tr>
                <tr>
                  <td>Profiles</td>
                  <td>30 days</td>
                </tr>
                <tr>
                  <td>Synthetic monitor results</td>
                  <td>90 days</td>
                </tr>
                <tr>
                  <td>
                    Traces, exceptions, sessions, session replays, AI
                    conversations
                  </td>
                  <td>
                    While the project exists. Deleted within 30 days after you
                    delete the project or close your account. On request we
                    apply a shorter window to your organization.
                  </td>
                </tr>
                <tr>
                  <td>Contact form submissions</td>
                  <td>12 months after our last exchange</td>
                </tr>
                <tr>
                  <td>Blog subscription</td>
                  <td>Until you unsubscribe</td>
                </tr>
                <tr>
                  <td>Google Analytics data</td>
                  <td>At most 14 months, at Google</td>
                </tr>
                <tr>
                  <td>Database backups</td>
                  <td>Rotated within 35 days</td>
                </tr>
              </tbody>
            </table>
          </div>

          <h2 id="security">10. Security</h2>
          <p>
            All traffic to our sites and APIs is encrypted with TLS. Data at
            rest on AWS is encrypted with AWS-managed keys. Passwords are
            stored as salted hashes, and every API token, personal access
            token and refresh token is stored as a SHA-256 hash, so a database
            copy does not reveal them. Production access is limited to the
            founders, uses SSH keys and multi-factor authentication, and
            changes ship through a reviewed, audited pipeline. Rate limits and
            admission controls protect the ingest endpoints, and private
            network targets are blocked for synthetic monitors on Cloud.
            Independent SOC 2 Type II and ISO 27001 audits are in progress.
            If we ever suffer a breach that affects your data, we will notify
            you without undue delay and the supervisory authority within 72
            hours where the law requires it.
          </p>

          <h2 id="rights">11. Your rights</h2>
          <p>Under the GDPR and similar laws you can ask us to:</p>
          <ul>
            <li>confirm whether we hold data about you and give you a copy (access)</li>
            <li>correct data that is wrong (rectification)</li>
            <li>delete your data (erasure)</li>
            <li>stop or limit processing (restriction and objection)</li>
            <li>hand your data over in a machine-readable format (portability)</li>
            <li>withdraw a consent you gave, with effect for the future</li>
          </ul>
          <p>
            Email <a href={`mailto:${LEGAL.privacyEmail}`}>{LEGAL.privacyEmail}</a>{" "}
            from the address on your account, or use the{" "}
            <Link href="/contact">contact form</Link>. We answer within one
            month, and we may ask you to confirm your identity first. Account
            holders can change their name, email and contact methods, revoke
            tokens, delete projects and cancel subscriptions from the
            dashboard; to close the account entirely, email us and we delete it
            within 30 days. You can also complain to a data protection
            authority, in particular the one in the EU member state where you
            live or work.
          </p>
          <p>
            <strong>If you live in California or another US state</strong>{" "}
            with a privacy law, you have equivalent rights to know, access,
            correct and delete your data, and to opt out of its sale or
            sharing. We do not sell personal data and do not share it for
            cross-context behavioral advertising, so there is nothing to opt
            out of. Exercise the other rights by email as described above; we
            will not treat you differently for doing so.
          </p>
          <p>
            <strong>
              If your data was sent to us by one of our customers
            </strong>{" "}
            (section 5), that customer is the controller and the one who can
            find, correct or delete it, because we cannot identify you in their
            telemetry. Please contact them. If you do write to us, we will
            forward your request to the customer where we can identify them,
            and we help every customer honor such requests.
          </p>

          <h2 id="children">12. Children</h2>
          <p>
            Our services are for businesses and developers and are not
            directed at anyone under 16. We do not knowingly collect data from
            children. If you believe a child has given us data, tell us and we
            will delete it.
          </p>

          <h2 id="automated">13. Automated decisions</h2>
          <p>
            We do not make decisions about you by automated means that have
            legal or similarly significant effects. Plan limits are enforced
            automatically, but they follow from the plan you chose and are
            explained in the dashboard.
          </p>

          <h2 id="self-hosted">14. Self-hosted Traceway</h2>
          <p>
            Traceway is open source and you can run it on your own servers.
            A self-hosted instance sends no data to us: no telemetry, no usage
            statistics, no crash reports. It only contacts third parties you
            configure yourself, such as a Slack webhook or GitHub for issue
            notifications. For a self-hosted instance, the operator is the
            controller and this policy does not apply.
          </p>

          <h2 id="changes">15. Changes to this policy</h2>
          <p>
            When we change this policy, we update the date at the top. If a
            change affects how we use the data of account holders in a
            material way, we also email account administrators at least 14
            days before it takes effect.
          </p>

          <h2 id="contact">16. Contact</h2>
          <p>
            {LEGAL.entity}, {LEGAL.address}. Email{" "}
            <a href={`mailto:${LEGAL.privacyEmail}`}>{LEGAL.privacyEmail}</a>{" "}
            or use the <Link href="/contact">contact form</Link>.
          </p>
        </article>
      </section>
    </main>
  );
}
