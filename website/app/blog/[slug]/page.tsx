import Link from "next/link";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { MDXRemote } from "next-mdx-remote/rsc";
import remarkGfm from "remark-gfm";
import { ArrowLeft } from "lucide-react";
import { Eyebrow } from "@/components/eyebrow";
import { BlogByline } from "@/components/blog-byline";
import { BlogSubscribe } from "@/components/blog-subscribe";
import { getAllPosts, getPostBySlug } from "@/lib/blog";

type Params = { slug: string };

// Note: "engineering" is reserved by the static /blog/engineering route, so no
// post may be named engineering.mdx; the static segment would shadow it here.
export function generateStaticParams(): Params[] {
  return getAllPosts().map((p) => ({ slug: p.slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<Params>;
}): Promise<Metadata> {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  if (!post) return { title: "Not found · Traceway" };
  const description =
    post.description ??
    (post.category === "engineering"
      ? `${post.title}, from the Traceway engineering blog.`
      : `Release notes for Traceway ${post.title}.`);
  return {
    title: `${post.title} · Traceway`,
    description,
  };
}

export default async function BlogPostPage({
  params,
}: {
  params: Promise<Params>;
}) {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  if (!post) notFound();

  const isEngineering = post.category === "engineering";

  return (
    <main className="relative">
      <section className="wrap pt-6 pb-24">
        <div className="blog-article">
          <Link
            href={isEngineering ? "/blog/engineering" : "/blog"}
            className="inline-flex items-center gap-1 text-[13px] mb-6"
            style={{
              color: "var(--fg-2)",
              textDecoration: "none",
              fontFamily: "var(--font-mono)",
            }}
          >
            <ArrowLeft className="h-3 w-3" />
            All posts
          </Link>

          <article className="blog-panel">
            <div className="prose">
              <Eyebrow>{isEngineering ? "Engineering" : "Release"}</Eyebrow>
              <h1 className="mt-4 mb-3">{post.title}</h1>
              <BlogByline date={formatDate(post.date)} />

              {post.description && (
                <p className="blog-lead mb-12">{post.description}</p>
              )}

              <div className="blog-body">
                <MDXRemote
                  source={post.content}
                  options={{ mdxOptions: { remarkPlugins: [remarkGfm] } }}
                />
              </div>
            </div>
          </article>

          {isEngineering && <BlogSubscribe />}
        </div>
      </section>
    </main>
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
