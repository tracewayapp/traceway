import Link from "next/link";
import { compileMDX } from "next-mdx-remote/rsc";
import remarkGfm from "remark-gfm";
import rehypePrettyCode, {
  type Options as RehypePrettyCodeOptions,
} from "rehype-pretty-code";
import { ArrowLeft } from "lucide-react";
import { Eyebrow } from "@/components/eyebrow";
import { BlogByline } from "@/components/blog-byline";
import { BlogCover } from "@/components/blog-cover";
import { BlogImage } from "@/components/blog-image";
import { BlogPostCard } from "@/components/blog-post-card";
import { BlogSubscribe } from "@/components/blog-subscribe";
import { BlogToc } from "@/components/blog-toc";
import { formatPostDate, type BlogPost, type BlogPostMeta } from "@/lib/blog";
import { rehypeBlogHeadings, type BlogHeading } from "@/lib/blog-headings";
import { tracewayShiki } from "@/lib/shiki-theme";

// A table of contents for two headings is noise.
const MIN_TOC_HEADINGS = 3;

export async function BlogArticle({
  post,
  backHref,
  eyebrow,
  showSubscribe,
  longForm = false,
  morePosts = [],
  codeTheme = tracewayShiki,
}: {
  post: BlogPost;
  backHref: string;
  eyebrow: string;
  showSubscribe: boolean;
  longForm?: boolean;
  morePosts?: BlogPostMeta[];
  codeTheme?: RehypePrettyCodeOptions["theme"];
}) {
  const prettyCodeOptions: RehypePrettyCodeOptions = {
    theme: codeTheme,
    keepBackground: false,
    defaultLang: "plaintext",
  };

  const headings: BlogHeading[] = [];
  const { content } = await compileMDX({
    source: post.content,
    components: { img: BlogImage },
    options: {
      mdxOptions: {
        remarkPlugins: [remarkGfm],
        rehypePlugins: [
          rehypeBlogHeadings((heading) => headings.push(heading)),
          [rehypePrettyCode, prettyCodeOptions],
        ],
      },
    },
  });
  const tocHeadings =
    longForm && headings.length >= MIN_TOC_HEADINGS ? headings : [];

  return (
    <main className="relative blog-scope">
      <section className="blog-wrap pt-12 pb-24">
        <div className="blog-top" data-category={post.category}>
          <Link href={backHref} className="blog-back">
            <span className="blog-back-icon">
              <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
            </span>
            All posts
          </Link>

          <header className="blog-header">
            <Eyebrow>{eyebrow}</Eyebrow>
            <h1 className="blog-title">{post.title}</h1>
            {post.description && (
              <p className="blog-lead">{post.description}</p>
            )}
            <BlogByline
              date={formatPostDate(post.date)}
              author={post.author}
              readingMinutes={longForm ? post.readingMinutes : undefined}
            />
          </header>
        </div>

        {longForm && post.cover?.startsWith("/") && (
          <BlogCover slug={post.slug} cover={post.cover} className="blog-hero" />
        )}

        <div className="blog-layout">
          <BlogToc headings={tocHeadings} />
          <article className="blog-body prose">{content}</article>
          {showSubscribe && (
            <div className="blog-after">
              <BlogSubscribe />
            </div>
          )}
        </div>

        {morePosts.length > 0 && (
          <div className="blog-wide blog-more">
            <div className="blog-more-head">
              <h2>Keep reading</h2>
              <Link href={backHref} className="blog-back">
                All posts
              </Link>
            </div>
            <div className="blog-grid">
              {morePosts.map((more) => (
                <BlogPostCard key={more.slug} post={more} />
              ))}
            </div>
          </div>
        )}
      </section>
    </main>
  );
}
