import type { Metadata } from "next";
import { Eyebrow } from "@/components/eyebrow";
import { BlogTabs } from "@/components/blog-tabs";
import { BlogPostList } from "@/components/blog-post-list";
import { BlogSubscribe } from "@/components/blog-subscribe";
import { getPostsByCategory } from "@/lib/blog";

export const metadata: Metadata = {
  title: "Engineering Blog · Traceway",
  description:
    "Deep dives, benchmarks, and engineering notes from the Traceway team.",
};

export default function EngineeringBlogIndex() {
  const posts = getPostsByCategory("engineering");

  return (
    <main className="relative">
      <section className="wrap pt-6 pb-24">
        <div className="prose max-w-[960px]">
          <Eyebrow>Blog</Eyebrow>
          <h1 className="mt-4 mb-3">Engineering</h1>
          <p style={{ color: "var(--fg-2)" }} className="mb-12">
            Deep dives, benchmarks, and how we build Traceway.
          </p>

          <BlogTabs active="engineering" />

          <BlogPostList posts={posts} />

          <BlogSubscribe />
        </div>
      </section>
    </main>
  );
}
