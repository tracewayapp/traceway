import type { Metadata } from "next";
import { Eyebrow } from "@/components/eyebrow";
import { BlogTabs } from "@/components/blog-tabs";
import { BlogPostList } from "@/components/blog-post-list";
import { getPostsByCategory } from "@/lib/blog";

export const metadata: Metadata = {
  title: "Releases · Traceway",
  description: "Release notes and updates from the Traceway team.",
};

export default function ReleasesIndex() {
  const posts = getPostsByCategory("release");

  return (
    <main className="relative">
      <section className="wrap pt-6 pb-24">
        <div className="prose max-w-[960px]">
          <Eyebrow>Blog</Eyebrow>
          <h1 className="mt-4 mb-3">Releases & updates</h1>
          <p style={{ color: "var(--fg-2)" }} className="mb-12">
            What we shipped, and when.
          </p>

          <BlogTabs active="releases" />

          <BlogPostList posts={posts} />
        </div>
      </section>
    </main>
  );
}
