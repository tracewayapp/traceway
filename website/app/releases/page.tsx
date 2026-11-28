import type { Metadata } from "next";
import { Eyebrow } from "@/components/eyebrow";
import { BlogTabs } from "@/components/blog-tabs";
import { BlogPostList } from "@/components/blog-post-list";
import { Holiday } from "@/components/holiday";
import { getPostsByCategory } from "@/lib/blog";

export const metadata: Metadata = {
  title: "Releases · Traceway",
  description: "Release notes and updates from the Traceway team.",
};

export default function ReleasesIndex() {
  const posts = getPostsByCategory("release");

  return (
    <main className="relative blog-scope">
      <section className="wrap pt-6 pb-24">
        <div className="relative hidden max-w-[960px] md:block">
          <Holiday
            src="/holiday/sled-of-gifts.svg"
            width={100}
            height={100}
            className="absolute right-0 top-0 size-24"
          />
        </div>
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
