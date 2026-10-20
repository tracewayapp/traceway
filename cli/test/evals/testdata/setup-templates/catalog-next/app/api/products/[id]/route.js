import { findProduct } from "@/lib/products";

export async function GET(request, { params }) {
  const { id } = await params;
  const product = await findProduct(id);
  if (!product) {
    return Response.json({ error: "product not found" }, { status: 404 });
  }
  return Response.json(product);
}
