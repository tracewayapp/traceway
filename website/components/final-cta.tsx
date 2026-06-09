import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { cn } from "@/lib/utils";

type Action = { label: string; href: string; external?: boolean };

export function FinalCTA({
  title,
  description,
  primary,
  secondary,
  className,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  primary: Action;
  secondary?: Action;
  className?: string;
}) {
  return (
    <section className={cn("wrap py-20", className)}>
      <div className="final-cta-box">
        <h2>{title}</h2>
        {description ? <p>{description}</p> : null}
        <div className="btns">
          <Link
            href={primary.href}
            className="btn btn-primary"
            {...(primary.external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
          >
            {primary.label}
            <ArrowRight className="h-4 w-4" />
          </Link>
          {secondary ? (
            <Link
              href={secondary.href}
              className="btn btn-ghost"
              {...(secondary.external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
            >
              {secondary.label}
            </Link>
          ) : null}
        </div>
      </div>
    </section>
  );
}
