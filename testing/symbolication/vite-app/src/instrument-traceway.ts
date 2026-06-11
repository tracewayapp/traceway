import { init } from "@tracewayapp/frontend";

const token =
  import.meta.env.VITE_TW_TOKEN || "5638c2de607f45169bcf98aa8774fe5c";
const reportUrl =
  import.meta.env.VITE_TW_REPORT_URL || "http://localhost:8082/api/report";

init(`${token}@${reportUrl}`, { debug: true });
