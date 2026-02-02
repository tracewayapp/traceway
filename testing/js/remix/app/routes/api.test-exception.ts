import { withTraceway } from "~/lib/traceway";

export const loader = withTraceway(async () => {
  throw new Error("test panic from /test-exception");
});
