import NextAuth from "next-auth";
import Credentials from "next-auth/providers/credentials";
import GitHub from "next-auth/providers/github";
import { verifyPassword } from "@/lib/passwords";
import { getUserByEmail } from "@/lib/users";

export const { handlers, auth, signIn, signOut } = NextAuth({
  // Auth.js only auto-trusts the request Host header on platforms it can
  // verify itself (Vercel). Self-hosted behind Docker/a reverse proxy —
  // this project's actual deployment shape — isn't one of those, so
  // without this every sign-in fails in production with "UntrustedHost"
  // even though the request is legitimate. Safe here specifically because
  // CONDUIT_MODE is always "test" and there's no proxy this service sits
  // behind that could spoof Host to a different, unintended value.
  trustHost: true,
  // JWT sessions, not database sessions: this app has exactly one table
  // (users) and no notion of a server-side session store — the session
  // itself is the signed token, same shape as every other short-lived
  // signed-token trust boundary in this project (conduit-core's internal
  // JWT, the dashboard-session JWT conduit-core verifies).
  session: { strategy: "jwt" },
  pages: { signIn: "/login" },
  providers: [
    Credentials({
      credentials: { email: {}, password: {} },
      async authorize(credentials) {
        const email =
          typeof credentials?.email === "string"
            ? credentials.email
            : undefined;
        const password =
          typeof credentials?.password === "string"
            ? credentials.password
            : undefined;
        if (!email || !password) {
          return null;
        }

        const user = await getUserByEmail(email);
        if (!user || !user.passwordHash) {
          return null;
        }
        if (!(await verifyPassword(user.passwordHash, password))) {
          return null;
        }

        return {
          id: user.id,
          email: user.email,
          merchantId: user.merchantId,
          role: user.role,
        };
      },
    }),
    // GitHub sign-in only ever links to an EXISTING dashboard account
    // (matched by email in the signIn callback below) — there is no fresh
    // signup via OAuth. A dashboard account only comes to exist after
    // proving ownership of a merchant's sk_test_... key (see
    // app/signup/actions.ts), and OAuth has no way to present that proof.
    GitHub,
  ],
  callbacks: {
    async signIn({ user, account }) {
      if (account?.provider !== "github") {
        return true;
      }
      if (!user.email) {
        return false;
      }
      const existing = await getUserByEmail(user.email);
      if (!existing) {
        return false;
      }
      user.id = existing.id;
      user.merchantId = existing.merchantId;
      user.role = existing.role;
      return true;
    },
    async jwt({ token, user }) {
      if (user) {
        token.sub = user.id;
        token.merchantId = user.merchantId;
        token.role = user.role;
      }
      return token;
    },
    async session({ session, token }) {
      session.user.id = token.sub as string;
      session.user.merchantId = token.merchantId;
      session.user.role = token.role;
      return session;
    },
  },
});
