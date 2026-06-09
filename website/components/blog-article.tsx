import Link from "next/link";
import { MDXRemote } from "next-mdx-remote/rsc";
import remarkGfm from "remark-gfm";
import rehypePrettyCode, {
  type Options as RehypePrettyCodeOptions,
} from "rehype-pretty-code";
import { ArrowLeft } from "lucide-react";
import { Eyebrow } from "@/components/eyebrow";
import { BlogByline } from "@/components/blog-byline";
import { BlogSubscribe } from "@/components/blog-subscribe";
import type { BlogPost } from "@/lib/blog";
import { tracewayShiki } from "@/lib/shiki-theme";

const prettyCodeOptions: RehypePrettyCodeOptions = {
  theme: tracewayShiki,
  keepBackground: false,
  defaultLang: "plaintext",
};

export function BlogArticle({
  post,
  backHref,
  eyebrow,
  showSubscribe,
}: {
  post: BlogPost;
  backHref: string;
  eyebrow: string;
  showSubscribe: boolean;
}) {
  return (
    <main className="relative">
      <section className="wrap pt-6 pb-24">
        <div className="blog-article">
          <Link
            href={backHref}
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
              <Eyebrow>{eyebrow}</Eyebrow>
              <h1 className="mt-4 mb-3">{post.title}</h1>
              <BlogByline date={formatDate(post.date)} author={post.author} />

              {post.description && (
                <p className="blog-lead mb-12">{post.description}</p>
              )}

              <div className="blog-body">
                <MDXRemote
                  source={post.content}
                  options={{
                    mdxOptions: {
                      remarkPlugins: [remarkGfm],
                      rehypePlugins: [[rehypePrettyCode, prettyCodeOptions]],
                    },
                  }}
                />
              </div>
            </div>
          </article>

          {showSubscribe && <BlogSubscribe />}
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
