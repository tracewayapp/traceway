import Link from "next/link";
import Image from "next/image";
import { BlogCover } from "@/components/blog-cover";
import { blogAuthor } from "@/components/blog-byline";
import { formatPostDate, postHref, type BlogPostMeta } from "@/lib/blog";

export function BlogPostCard({
  post,
  featured = false,
}: {
  post: BlogPostMeta;
  featured?: boolean;
}) {
  const author = blogAuthor(post.author);
  return (
    <Link
      href={postHref(post)}
      className={`blog-post-card${featured ? " blog-post-card-featured" : ""}`}
    >
      <BlogCover slug={post.slug} cover={post.cover} />
      <div className="blog-post-card-body">
        <div className="blog-post-card-meta">
          {formatPostDate(post.date)} · {post.readingMinutes} min read
        </div>
        <h2 className="blog-post-card-title">{post.title}</h2>
        {post.description && (
          <p className="blog-post-card-desc">{post.description}</p>
        )}
        <div className="blog-post-card-author">
          <Image src={author.avatar} alt="" width={24} height={24} />
          {author.name}
        </div>
      </div>
    </Link>
  );
}

export function BlogPostGrid({ posts }: { posts: BlogPostMeta[] }) {
  if (posts.length === 0) {
    return <p style={{ color: "var(--fg-3)" }}>No posts yet.</p>;
  }
  const [latest, ...rest] = posts;
  return (
    <>
      <BlogPostCard post={latest} featured />
      <div className="blog-grid">
        {rest.map((post) => (
          <BlogPostCard key={post.slug} post={post} />
        ))}
      </div>
    </>
  );
}
