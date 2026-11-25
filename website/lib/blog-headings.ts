export type BlogHeading = {
  id: string;
  text: string;
  depth: 2 | 3;
};

type HastNode = {
  type: string;
  tagName?: string;
  value?: string;
  properties?: Record<string, unknown>;
  children?: HastNode[];
};

function textOf(node: HastNode): string {
  if (node.type === "text") return node.value ?? "";
  return (node.children ?? []).map(textOf).join("");
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

/**
 * Rehype plugin: gives every h2/h3 a stable id plus a hover anchor, and
 * reports them to `collect` in document order. The table of contents is built
 * from that list, so it can never drift from the ids in the rendered article.
 */
export function rehypeBlogHeadings(collect: (heading: BlogHeading) => void) {
  return () => (tree: HastNode) => {
    const used = new Map<string, number>();
    const visit = (node: HastNode) => {
      if (
        node.type === "element" &&
        (node.tagName === "h2" || node.tagName === "h3")
      ) {
        const text = textOf(node).trim();
        const base = slugify(text) || "section";
        const seen = used.get(base) ?? 0;
        used.set(base, seen + 1);
        const id = seen === 0 ? base : `${base}-${seen + 1}`;
        node.properties = { ...node.properties, id };
        node.children = [
          ...(node.children ?? []),
          {
            type: "element",
            tagName: "a",
            properties: {
              href: `#${id}`,
              className: ["blog-anchor"],
              ariaLabel: `Link to the section “${text}”`,
            },
            children: [{ type: "text", value: "#" }],
          },
        ];
        collect({ id, text, depth: node.tagName === "h2" ? 2 : 3 });
        return;
      }
      (node.children ?? []).forEach(visit);
    };
    visit(tree);
  };
}
