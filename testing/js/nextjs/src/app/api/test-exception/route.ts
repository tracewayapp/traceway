import { withTraceway } from "@/lib/traceway-handler";

export const GET = withTraceway(async () => {
  throw new Error("test panic from /test-exception");
});
