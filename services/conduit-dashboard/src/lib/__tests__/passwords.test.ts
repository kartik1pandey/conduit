import { describe, expect, it } from "vitest";
import { hashPassword, verifyPassword } from "@/lib/passwords";

describe("passwords", () => {
  it("round-trips a correct password", async () => {
    const hash = await hashPassword("correct-horse-battery-staple");
    expect(await verifyPassword(hash, "correct-horse-battery-staple")).toBe(
      true,
    );
  });

  it("rejects an incorrect password", async () => {
    const hash = await hashPassword("correct-horse-battery-staple");
    expect(await verifyPassword(hash, "wrong-password")).toBe(false);
  });

  it("never stores the plaintext in the hash", async () => {
    const hash = await hashPassword("correct-horse-battery-staple");
    expect(hash).not.toContain("correct-horse-battery-staple");
  });

  it("treats a malformed hash as a verification failure, not a crash", async () => {
    await expect(verifyPassword("not-a-real-hash", "anything")).resolves.toBe(
      false,
    );
  });
});
