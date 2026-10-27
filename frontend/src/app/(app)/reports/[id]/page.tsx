import { EditReport } from "./edit/edit-report";

export const metadata = { title: "View report" };

export const instant = false;

export default function Page() {
  return <EditReport viewOnly />;
}
