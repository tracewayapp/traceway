import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { BlogArticle } from "@/components/blog-article";
import { getPostsByCategory, getPostBySlug, postMetadata } from "@/lib/blog";
import { vscodeDarkShiki } from "@/lib/shiki-theme-vscode";

type Params = { slug: string };

export function generateStaticParams(): Params[] {
  return getPostsByCategory("engineering").map((p) => ({ slug: p.slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<Params>;
}): Promise<Metadata> {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  if (!post || post.category !== "engineering")
    return { title: "Not found · Traceway" };
  return postMetadata(post);
}

export default async function BlogPostPage({
  params,
}: {
  params: Promise<Params>;
}) {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  if (!post || post.category !== "engineering") notFound();

  return (
    <BlogArticle
      post={post}
      backHref="/blog"
      eyebrow="Engineering"
      showSubscribe
      codeTheme={vscodeDarkShiki}
    />
  );
}
