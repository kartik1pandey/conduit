import argon2 from "argon2";

// argon2id is the mode CLAUDE.md's non-negotiables call out by name
// ("hashed with argon2 or bcrypt") and the mode OWASP currently recommends
// over plain argon2i/argon2d — resistant to both GPU-cracking and
// side-channel attacks, which matters for a password hash more than for
// most other uses of a hash function.
export async function hashPassword(plaintext: string): Promise<string> {
  return argon2.hash(plaintext, { type: argon2.argon2id });
}

export async function verifyPassword(
  hash: string,
  plaintext: string,
): Promise<boolean> {
  try {
    return await argon2.verify(hash, plaintext);
  } catch {
    // A malformed hash (shouldn't happen, but never trust stored data
    // blindly) is a verification failure, not a crash.
    return false;
  }
}
