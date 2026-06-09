import Link from "next/link";

type Tab = { label: string; href: string; key: "blog" | "releases" };

const TABS: Tab[] = [
  { label: "Blog", href: "/blog", key: "blog" },
  { label: "Releases", href: "/releases", key: "releases" },
];

export function BlogTabs({ active }: { active: "blog" | "releases" }) {
  return (
    <div className="mb-12">
      <nav className="blog-tabs">
        {TABS.map((tab) => (
          <Link
            key={tab.key}
            href={tab.href}
            aria-current={tab.key === active ? "page" : undefined}
            className={`blog-tab${tab.key === active ? " blog-tab-active" : ""}`}
          >
            {tab.label}
          </Link>
        ))}
      </nav>
    </div>
  );
}
