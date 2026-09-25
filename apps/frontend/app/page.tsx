import { redirect } from "next/navigation";

// The app has no public landing page: the shell decides where an anonymous visitor goes.
export default function HomePage() {
  redirect("/dashboard");
}
