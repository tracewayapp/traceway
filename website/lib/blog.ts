import fs from "node:fs";
import path from "node:path";
import matter from "gray-matter";

const BLOG_DIR = path.join(process.cwd(), "content", "blog");

export type BlogCategory = "release" | "engineering";

export type BlogPostMeta = {
  slug: string;
  title: string;
  date: string;
  version?: string;
  category: BlogCategory;
  description?: string;
};

export type BlogPost = BlogPostMeta & {
  content: string;
};

function readPostFile(filename: string): BlogPost | null {
  if (!filename.endsWith(".mdx")) return null;
  const slug = filename.replace(/\.mdx$/, "");
  const raw = fs.readFileSync(path.join(BLOG_DIR, filename), "utf8");
  const { data, content } = matter(raw);
  const title = typeof data.title === "string" ? data.title : slug;
  const date = typeof data.date === "string" ? data.date : "";
  const version = typeof data.version === "string" ? data.version : undefined;
  const category: BlogCategory =
    data.category === "engineering" ? "engineering" : "release";
  const description =
    typeof data.description === "string" ? data.description : undefined;
  return { slug, title, date, version, category, description, content };
}

export function getAllPosts(): BlogPostMeta[] {
  if (!fs.existsSync(BLOG_DIR)) return [];
  return fs
    .readdirSync(BLOG_DIR)
    .map(readPostFile)
    .filter((p): p is BlogPost => p !== null)
    .sort((a, b) => (a.date < b.date ? 1 : -1))
    .map(({ content: _content, ...meta }) => meta);
}

export function getPostsByCategory(category: BlogCategory): BlogPostMeta[] {
  return getAllPosts().filter((p) => p.category === category);
}

export function getPostBySlug(slug: string): BlogPost | null {
  const filename = `${slug}.mdx`;
  const filepath = path.join(BLOG_DIR, filename);
  if (!fs.existsSync(filepath)) return null;
  return readPostFile(filename);
}

export function postHref(p: { slug: string; category: BlogCategory }): string {
  return p.category === "engineering"
    ? `/blog/${p.slug}`
    : `/releases/${p.slug}`;
}

export function postMetadata(post: BlogPost): {
  title: string;
  description: string;
} {
  const description =
    post.description ??
    (post.category === "engineering"
      ? `${post.title}, from the Traceway engineering blog.`
      : `Release notes for Traceway ${post.title}.`);
  return { title: `${post.title} · Traceway`, description };
}
