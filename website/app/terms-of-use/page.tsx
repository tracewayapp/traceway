import Link from "next/link";

export default function TermsOfUse() {
  return (
    <main className="min-h-screen bg-white text-zinc-950 font-sans">
      <div className="container mx-auto px-4 max-w-3xl py-20">
        <h1 className="text-4xl font-bold tracking-tight mb-2 text-zinc-900">
          Terms of Use
        </h1>
        <p className="text-zinc-500 mb-12">Last updated: February 15, 2026</p>

        <div className="space-y-10 text-zinc-700 leading-relaxed">
          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              1. Acceptance of Terms
            </h2>
            <p>
              By accessing or using Traceway Cloud (&ldquo;the Service&rdquo;),
              you agree to be bound by these Terms of Use. If you do not agree
              to these terms, you may not use the Service. These terms apply to
              the Traceway Cloud offering operated by Traceway (&ldquo;we&rdquo;,
              &ldquo;our&rdquo;, or &ldquo;us&rdquo;).
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              2. Description of Service
            </h2>
            <p>
              Traceway Cloud is a hosted application monitoring platform that
              provides endpoint analytics, exception tracking, session replay,
              and Impact Score prioritization. The Service ingests telemetry data
              from your applications via the Traceway SDK or OpenTelemetry and
              presents it through a web dashboard.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              3. Account Responsibilities
            </h2>
            <p>
              You are responsible for maintaining the confidentiality of your
              account credentials and project tokens. You are responsible for all
              activity that occurs under your account. You agree to notify us
              immediately of any unauthorized use of your account.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              4. Acceptable Use
            </h2>
            <p>You agree not to:</p>
            <ul className="list-disc pl-5 space-y-2 mt-2">
              <li>
                Use the Service for any unlawful purpose or in violation of any
                applicable laws
              </li>
              <li>
                Attempt to gain unauthorized access to the Service or its
                related systems
              </li>
              <li>
                Interfere with or disrupt the integrity or performance of the
                Service
              </li>
              <li>
                Transmit any malicious code, viruses, or harmful data through
                the Service
              </li>
              <li>
                Resell, sublicense, or redistribute the Service without our
                prior written consent
              </li>
            </ul>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              5. Open-Source License Disclaimer
            </h2>
            <p>
              Traceway is available as open-source software for self-hosting.
              The self-hosted version is provided &ldquo;as is&rdquo; without
              warranty of any kind, express or implied, including but not limited
              to the warranties of merchantability, fitness for a particular
              purpose, and non-infringement. Traceway takes no responsibility
              for any issues, data loss, security vulnerabilities, or downtime
              arising from customer self-hosted deployments. You are solely
              responsible for the operation, maintenance, and security of your
              self-hosted instance.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              6. Intellectual Property
            </h2>
            <p>
              The Traceway name, logo, and branding are the property of
              Traceway. The Traceway Cloud platform design, proprietary
              infrastructure, and service delivery are owned by us. The
              open-source codebase is licensed under its respective open-source
              license.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              7. Limitation of Liability
            </h2>
            <p>
              To the maximum extent permitted by applicable law, Traceway shall
              not be liable for any indirect, incidental, special,
              consequential, or punitive damages, or any loss of profits or
              revenues, whether incurred directly or indirectly, or any loss of
              data, use, goodwill, or other intangible losses resulting from
              your use of or inability to use the Service.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              8. Service Availability
            </h2>
            <p>
              We strive to maintain high availability of Traceway Cloud but do
              not guarantee uninterrupted access. We may perform scheduled
              maintenance with reasonable notice. We are not liable for any
              downtime or service interruptions.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              9. Termination
            </h2>
            <p>
              We may suspend or terminate your access to the Service at any time
              for violation of these terms or for any other reason at our sole
              discretion. You may terminate your account at any time by
              contacting us. Upon termination, your right to use the Service
              ceases immediately. We may delete your data after a reasonable
              retention period following termination.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              10. Changes to Terms
            </h2>
            <p>
              We reserve the right to modify these Terms of Use at any time. We
              will notify you of material changes by posting the updated terms
              on this page with a revised &ldquo;Last updated&rdquo; date.
              Continued use of the Service after changes constitutes acceptance
              of the new terms.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">
              11. Contact
            </h2>
            <p>
              If you have any questions about these Terms of Use, please{" "}
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
