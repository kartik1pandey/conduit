import { ImageResponse } from "next/og";

export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

// Rendered from JSX/CSS at request time — no raster asset pipeline
// needed, and it can't drift out of sync with the landing page's own
// dark/mesh treatment since both come from the same color values.
export default function OpengraphImage() {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          padding: 80,
          background:
            "radial-gradient(circle at 20% 20%, rgba(124,108,245,0.55), transparent 55%), radial-gradient(circle at 85% 75%, rgba(52,211,153,0.35), transparent 50%), #0f0f17",
          color: "#ffffff",
          fontFamily: "sans-serif",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 12,
            marginBottom: 32,
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              width: 44,
              height: 44,
              borderRadius: 10,
              background: "#6250ee",
              fontSize: 24,
              fontWeight: 700,
            }}
          >
            C
          </div>
          <div style={{ fontSize: 28, fontWeight: 600 }}>Conduit</div>
        </div>
        <div
          style={{
            fontSize: 56,
            fontWeight: 700,
            lineHeight: 1.1,
            maxWidth: 900,
          }}
        >
          Payments infrastructure, built like it has to be right.
        </div>
        <div
          style={{
            marginTop: 24,
            fontSize: 24,
            color: "rgba(255,255,255,0.65)",
            maxWidth: 800,
          }}
        >
          Real-time risk scoring, a double-entry ledger, and reliably-delivered
          webhooks. Test mode — no real money ever moves.
        </div>
      </div>
    ),
    { ...size },
  );
}
