import Link from "next/link";

export default function PrivacyPolicy() {
  return (
    <main className="min-h-screen bg-white text-zinc-950 font-sans">
      <div className="container mx-auto px-4 max-w-3xl py-20">
        <h1 className="text-4xl font-bold tracking-tight mb-2 text-zinc-900">
          Privacy Policy
        </h1>
        <p className="text-zinc-500 mb-12">Last updated: February 15, 2026</p>

        <div className="space-y-10 text-zinc-700 leading-relaxed">
          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              1. Introduction
            </h2>
            <p>
              Traceway (&ldquo;we&rdquo;, &ldquo;our&rdquo;, or
              &ldquo;us&rdquo;) operates the Traceway Cloud platform and the
              tracewayapp.com website. This Privacy Policy explains how we
              collect, use, and protect your information when you use our
              services.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              2. Information We Collect
            </h2>
            <h3 className="font-medium text-zinc-900 mt-4 mb-2">
              Account Data
            </h3>
            <p>
              When you create an account, we collect your name, email address,
              and organization name. This information is stored in our PostgreSQL
              database and is used to manage your account and provide access to
              your projects.
            </p>
            <h3 className="font-medium text-zinc-900 mt-4 mb-2">
              Telemetry Data
            </h3>
            <p>
              When you integrate the Traceway SDK or send OpenTelemetry data to
              Traceway Cloud, we receive application telemetry including HTTP
              request metadata (endpoints, status codes, response times),
              exception stack traces, session replay recordings, and system
              metrics (CPU, memory usage). This data is stored in ClickHouse and
              is scoped to your project.
            </p>
            <h3 className="font-medium text-zinc-900 mt-4 mb-2">Usage Data</h3>
            <p>
              We may collect basic analytics about how you interact with the
              Traceway dashboard, such as pages visited and features used. This
              helps us improve the product experience.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              3. How We Use Your Data
            </h2>
            <ul className="list-disc pl-5 space-y-2">
              <li>To provide and operate the Traceway Cloud platform</li>
              <li>
                To display your application&apos;s telemetry data in the
                dashboard
              </li>
              <li>To calculate Impact Scores and prioritize issues</li>
              <li>To send important service-related communications</li>
              <li>To improve and develop new features</li>
            </ul>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              4. Data Retention
            </h2>
            <p>
              Telemetry data (transactions, exceptions, metrics) is retained
              according to your plan&apos;s retention period. Account data is
              retained for as long as your account is active. You may request
              deletion of your account and associated data at any time by
              contacting us.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              5. Data Security
            </h2>
            <p>
              We implement appropriate technical and organizational measures to
              protect your data. All data in transit is encrypted via TLS.
              Access to production systems is restricted and audited.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              6. Third-Party Services
            </h2>
            <p>
              Traceway Cloud may use third-party infrastructure providers for
              hosting and data storage. We do not sell your data to third
              parties. We may use service providers for email delivery and
              payment processing, who are bound by their own privacy policies.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              7. Open-Source Self-Hosted Version
            </h2>
            <p>
              Traceway is available as an open-source, self-hosted solution.
              When you self-host Traceway, all data remains entirely on your own
              infrastructure. We do not collect any data from self-hosted
              instances. The self-hosted version is provided &ldquo;as is&rdquo;
              with no guarantees regarding data handling, security, or
              availability. You are solely responsible for the configuration,
              security, and maintenance of your self-hosted deployment.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              8. Your Rights
            </h2>
            <p>
              You have the right to access, correct, or delete your personal
              data. You may also request an export of your data. To exercise
              these rights, please contact us using the information below.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              9. Changes to This Policy
            </h2>
            <p>
              We may update this Privacy Policy from time to time. We will
              notify you of any material changes by posting the updated policy on
              this page with a revised &ldquo;Last updated&rdquo; date.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              10. Contact
            </h2>
            <p>
              If you have any questions about this Privacy Policy, please{" "}
              <Link
                href="/contact"
                className="text-zinc-900 underline hover:text-zinc-600 transition-colors"
              >
                contact us
              </Link>
              .
            </p>
          </section>
        </div>
      </div>
    </main>
  );
}
