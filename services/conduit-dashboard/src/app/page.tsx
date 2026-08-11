import Link from "next/link";
import { redirect } from "next/navigation";
import { ArrowRight, ExternalLink } from "lucide-react";
import { auth } from "@/auth";
import { Nav } from "@/components/landing/Nav";
import { Hero } from "@/components/landing/Hero";
import { Stats } from "@/components/landing/Stats";
import { FlowDiagram } from "@/components/landing/FlowDiagram";
import { FeatureGrid } from "@/components/landing/FeatureGrid";
import { SmoothScroll } from "@/components/landing/SmoothScroll";

export default async function Home() {
  const session = await auth();
  if (session) {
    redirect("/dashboard");
  }

  return (
    <SmoothScroll>
      <div className="landing bg-[var(--l-bg)] text-[var(--l-foreground)]">
        <Nav />
        <Hero />
        <Stats />

        <section id="flow" className="px-6 py-24">
          <div className="mx-auto max-w-5xl">
            <h2 className="font-display text-2xl font-semibold sm:text-3xl">
              How a payment actually moves.
            </h2>
            <p className="mt-2 max-w-lg text-sm text-[var(--l-muted)]">
              Four services, each independently deployable, each talking to the
              others only over an authenticated API — never a shared database.
            </p>
            <FlowDiagram />
          </div>
        </section>

        <FeatureGrid />

        <section className="px-6 py-24 text-center">
          <h2 className="font-display text-2xl font-semibold sm:text-3xl">
            Try it — nothing to lose, nothing real to move.
          </h2>
          <p className="mx-auto mt-2 max-w-md text-sm text-[var(--l-muted)]">
            Claim a merchant, create a payment, and watch it flow through risk
            scoring and the ledger live.
          </p>
          <Link
            href="/signup"
            className="mt-6 inline-flex items-center gap-1.5 rounded-[var(--radius-sm)] bg-[var(--l-accent)] px-5 py-2.5 text-sm font-medium text-white transition hover:opacity-90"
          >
            Get started
            <ArrowRight className="size-4" strokeWidth={1.75} />
          </Link>
        </section>

        <footer className="border-t border-[var(--l-border)] px-6 py-8">
          <div className="mx-auto flex max-w-5xl flex-col items-center justify-between gap-3 text-xs text-[var(--l-muted)] sm:flex-row">
            <p>
              Conduit is a systems-design portfolio project — not a real payment
              processor. Test mode only, always.
            </p>
            <a
              href="https://github.com/kartik1pandey/conduit"
              className="inline-flex items-center gap-1.5 transition hover:text-[var(--l-foreground)]"
            >
              <ExternalLink className="size-3.5" strokeWidth={1.75} />
              Source
            </a>
          </div>
        </footer>
      </div>
    </SmoothScroll>
  );
}
