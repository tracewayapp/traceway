import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { BlogArticle } from "@/components/blog-article";
import { getPostsByCategory, getPostBySlug, postMetadata } from "@/lib/blog";

type Params = { slug: string };

export function generateStaticParams(): Params[] {
  return getPostsByCategory("release").map((p) => ({ slug: p.slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<Params>;
}): Promise<Metadata> {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  if (!post || post.category !== "release")
    return { title: "Not found · Traceway" };
  return postMetadata(post);
}

export default async function ReleasePostPage({
  params,
}: {
  params: Promise<Params>;
}) {
  const { slug } = await params;
  const post = getPostBySlug(slug);
  if (!post || post.category !== "release") notFound();

  return (
    <BlogArticle
      post={post}
      backHref="/releases"
      eyebrow="Release"
      showSubscribe={false}
    />
  );
}
