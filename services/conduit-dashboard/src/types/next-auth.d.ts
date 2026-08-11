import type { DefaultSession } from "next-auth";
import type { Role } from "@/lib/users";

// Augments Auth.js's built-in types with the two fields every page/server
// action in this app actually needs from a session: which merchant this
// dashboard user belongs to, and what they're allowed to do (enforced for
// real via policies/dashboard.rego, not by this type alone).
declare module "next-auth" {
  interface Session {
    user: {
      id: string;
      merchantId: string;
      role: Role;
    } & DefaultSession["user"];
  }

  interface User {
    merchantId: string;
    role: Role;
  }
}

// @auth/core/jwt (re-exported as next-auth/jwt) is where the JWT interface
// is actually declared — augmenting the re-export path alone doesn't merge
// into it, since it extends Record<string, unknown> under the hood and TS
// declaration merging targets the original declaring module.
declare module "@auth/core/jwt" {
  interface JWT {
    merchantId: string;
    role: Role;
  }
}
