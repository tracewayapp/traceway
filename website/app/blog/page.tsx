import type { Metadata } from "next";
import { Eyebrow } from "@/components/eyebrow";
import { BlogTabs } from "@/components/blog-tabs";
import { BlogPostGrid } from "@/components/blog-post-card";
import { BlogSubscribe } from "@/components/blog-subscribe";
import { Holiday } from "@/components/holiday";
import { getPostsByCategory } from "@/lib/blog";

export const metadata: Metadata = {
  title: "Blog · Traceway",
  description:
    "Deep dives, benchmarks, and engineering notes from the Traceway team.",
};

export default function BlogIndex() {
  const posts = getPostsByCategory("engineering");

  return (
    <main className="relative blog-scope">
      <section className="wrap relative pt-10 pb-24">
        <div className="blog-index relative">
          <Holiday
            src="/holiday/letters-for-santa.svg"
            width={100}
            height={100}
            className="absolute right-0 top-0 hidden size-24 md:block"
          />
          <Eyebrow>Blog</Eyebrow>
          <h1 className="blog-index-title">Engineering</h1>
          <p className="blog-lead mb-10" style={{ marginTop: 0 }}>
            Deep dives, benchmarks, and how we build Traceway.
          </p>

          <BlogTabs active="blog" />

          <BlogPostGrid posts={posts} />

          <BlogSubscribe />
        </div>
      </section>
    </main>
  );
}
