import Link from "next/link";
import { postHref, type BlogPostMeta } from "@/lib/blog";

export function BlogPostList({ posts }: { posts: BlogPostMeta[] }) {
  if (posts.length === 0) {
    return <p style={{ color: "var(--fg-3)" }}>No posts yet.</p>;
  }

  return (
    <ul
      style={{ listStyle: "none", padding: 0, margin: 0 }}
      className="flex flex-col gap-3"
    >
      {posts.map((post) => (
        <li key={post.slug}>
          <Link href={postHref(post)} className="blog-card group">
            <div
              className="text-[12px] mb-1.5"
              style={{ color: "var(--fg-3)", fontFamily: "var(--font-mono)" }}
            >
              {formatDate(post.date)}
            </div>
            <div
              className="text-[18px] font-medium transition-colors group-hover:text-[color:var(--fg-0)]"
              style={{ fontFamily: "var(--font-body)", color: "var(--fg-1)" }}
            >
              {post.title}
            </div>
            {post.description && (
              <p
                className="mt-2 text-[14px] leading-relaxed line-clamp-2"
                style={{ color: "var(--fg-2)", fontFamily: "var(--font-body)" }}
              >
                {post.description}
              </p>
            )}
          </Link>
        </li>
      ))}
    </ul>
  );
}

function formatDate(isoDate: string): string {
  if (!isoDate) return "";
  const d = new Date(isoDate + "T00:00:00Z");
  if (Number.isNaN(d.getTime())) return isoDate;
  return d.toLocaleDateString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
    timeZone: "UTC",
  });
}
